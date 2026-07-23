package data_svc

import (
	"context"
	"math"
	"sort"
	"strings"
	"sync"
)

// MallWeatherMetricDefinition describes a registry-agnostic metric contract.
// A Prometheus, OpenTelemetry, or logging-backed exporter should use these
// definitions as the canonical mall weather metric names and label sets.
type MallWeatherMetricDefinition struct {
	Name   string
	Labels []string
}

const (
	MallWeatherMetricFetchTotal               = "mall_weather_fetch_total"
	MallWeatherMetricFetchDurationSeconds     = "mall_weather_fetch_duration_seconds"
	MallWeatherMetricProviderRequestsTotal    = "mall_weather_provider_requests_total"
	MallWeatherMetricProviderRateLimitedTotal = "mall_weather_provider_rate_limited_total"
	MallWeatherMetricProviderCircuitOpen      = "mall_weather_provider_circuit_open"
	MallWeatherMetricDataAgeSeconds           = "mall_weather_data_age_seconds"
	MallWeatherMetricParseWarningsTotal       = "mall_weather_parse_warnings_total"
	MallWeatherMetricQueueLagSeconds          = "mall_weather_queue_lag_seconds"
	MallWeatherMetricExportRowsTotal          = "mall_weather_export_rows_total"
	MallWeatherMetricFeishuRowsTotal          = "mall_weather_feishu_rows_total"
	mallWeatherMetricStatusSuccess            = "success"
	mallWeatherMetricStatusFailed             = "failed"
)

var mallWeatherMetricDefinitions = []MallWeatherMetricDefinition{
	{Name: MallWeatherMetricFetchTotal, Labels: []string{"kind", "status"}},
	{Name: MallWeatherMetricFetchDurationSeconds, Labels: []string{"kind"}},
	{Name: MallWeatherMetricProviderRequestsTotal, Labels: []string{"endpoint", "status"}},
	{Name: MallWeatherMetricProviderRateLimitedTotal},
	{Name: MallWeatherMetricProviderCircuitOpen},
	{Name: MallWeatherMetricDataAgeSeconds, Labels: []string{"kind"}},
	{Name: MallWeatherMetricParseWarningsTotal, Labels: []string{"field"}},
	{Name: MallWeatherMetricQueueLagSeconds, Labels: []string{"kind"}},
	{Name: MallWeatherMetricExportRowsTotal},
	{Name: MallWeatherMetricFeishuRowsTotal, Labels: []string{"status"}},
}

func MallWeatherMetricDefinitions() []MallWeatherMetricDefinition {
	definitions := make([]MallWeatherMetricDefinition, len(mallWeatherMetricDefinitions))
	for index, definition := range mallWeatherMetricDefinitions {
		definitions[index] = MallWeatherMetricDefinition{
			Name:   definition.Name,
			Labels: append([]string(nil), definition.Labels...),
		}
	}
	return definitions
}

type mallWeatherMetricRecorder interface {
	AddCounter(name string, labels map[string]string, value int64)
}

type noopMallWeatherMetricRecorder struct{}

func (noopMallWeatherMetricRecorder) AddCounter(string, map[string]string, int64) {}

type MallWeatherMetricCounterSample struct {
	Name   string            `json:"name"`
	Labels map[string]string `json:"labels,omitempty"`
	Value  int64             `json:"value"`
}

type MallWeatherMetricsResult struct {
	Definitions []MallWeatherMetricDefinition    `json:"definitions"`
	Counters    []MallWeatherMetricCounterSample `json:"counters"`
}

type mallWeatherMetricSnapshotter interface {
	CounterSnapshot() []MallWeatherMetricCounterSample
}

type MallWeatherMetricsService struct {
	metrics mallWeatherMetricSnapshotter
}

func NewMallWeatherMetricsService() *MallWeatherMetricsService {
	return &MallWeatherMetricsService{metrics: mallWeatherRuntimeMetrics}
}

func newMallWeatherMetricsServiceWithRecorder(metrics mallWeatherMetricSnapshotter) (*MallWeatherMetricsService, error) {
	if metrics == nil {
		return nil, ErrMallWeatherInvalidQuery
	}
	return &MallWeatherMetricsService{metrics: metrics}, nil
}

func (service *MallWeatherMetricsService) Snapshot(
	ctx context.Context,
	actorID uint,
) (*MallWeatherMetricsResult, error) {
	if service == nil || service.metrics == nil || ctx == nil {
		return nil, ErrMallWeatherInvalidQuery
	}
	if actorID == 0 {
		return nil, ErrMallForbidden
	}
	return &MallWeatherMetricsResult{
		Definitions: MallWeatherMetricDefinitions(),
		Counters:    service.metrics.CounterSnapshot(),
	}, nil
}

type inMemoryMallWeatherMetricRecorder struct {
	mu       sync.RWMutex
	counters map[string]MallWeatherMetricCounterSample
}

func newInMemoryMallWeatherMetricRecorder() *inMemoryMallWeatherMetricRecorder {
	return &inMemoryMallWeatherMetricRecorder{
		counters: make(map[string]MallWeatherMetricCounterSample),
	}
}

func (recorder *inMemoryMallWeatherMetricRecorder) AddCounter(
	name string,
	labels map[string]string,
	value int64,
) {
	if recorder == nil || name == "" || value <= 0 {
		return
	}
	key := mallWeatherMetricSeriesKey(name, labels)
	labelCopy := copyMallWeatherMetricLabels(labels)

	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	if recorder.counters == nil {
		recorder.counters = make(map[string]MallWeatherMetricCounterSample)
	}
	sample := recorder.counters[key]
	if sample.Name == "" {
		sample.Name = name
		sample.Labels = labelCopy
	}
	if math.MaxInt64-sample.Value < value {
		sample.Value = math.MaxInt64
	} else {
		sample.Value += value
	}
	recorder.counters[key] = sample
}

func (recorder *inMemoryMallWeatherMetricRecorder) CounterSnapshot() []MallWeatherMetricCounterSample {
	if recorder == nil {
		return nil
	}
	recorder.mu.RLock()
	defer recorder.mu.RUnlock()

	samples := make([]MallWeatherMetricCounterSample, 0, len(recorder.counters))
	for _, sample := range recorder.counters {
		sample.Labels = copyMallWeatherMetricLabels(sample.Labels)
		samples = append(samples, sample)
	}
	sort.Slice(samples, func(left, right int) bool {
		leftKey := mallWeatherMetricSeriesKey(samples[left].Name, samples[left].Labels)
		rightKey := mallWeatherMetricSeriesKey(samples[right].Name, samples[right].Labels)
		return leftKey < rightKey
	})
	return samples
}

func mallWeatherMetricSeriesKey(name string, labels map[string]string) string {
	if len(labels) == 0 {
		return name
	}
	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var builder strings.Builder
	builder.WriteString(name)
	for _, key := range keys {
		builder.WriteByte(0)
		builder.WriteString(key)
		builder.WriteByte('=')
		builder.WriteString(labels[key])
	}
	return builder.String()
}

func copyMallWeatherMetricLabels(labels map[string]string) map[string]string {
	if len(labels) == 0 {
		return nil
	}
	copied := make(map[string]string, len(labels))
	for key, value := range labels {
		copied[key] = value
	}
	return copied
}

func recordMallWeatherFeishuRows(
	recorder mallWeatherMetricRecorder,
	result MallWeatherFeishuExecutionResult,
) {
	if recorder == nil {
		return
	}
	if result.SuccessCount > 0 {
		recorder.AddCounter(
			MallWeatherMetricFeishuRowsTotal,
			map[string]string{"status": mallWeatherMetricStatusSuccess},
			int64(result.SuccessCount),
		)
	}
}

func recordMallWeatherExportRows(
	recorder mallWeatherMetricRecorder,
	result MallWeatherExportRenderResult,
) {
	if recorder == nil || result.ProcessedRows <= 0 {
		return
	}
	recorder.AddCounter(MallWeatherMetricExportRowsTotal, nil, result.ProcessedRows)
}

func recordMallWeatherFetch(
	recorder mallWeatherMetricRecorder,
	taskKind string,
	status string,
) {
	if recorder == nil || taskKind == "" || status == "" {
		return
	}
	recorder.AddCounter(
		MallWeatherMetricFetchTotal,
		map[string]string{"kind": taskKind, "status": status},
		1,
	)
}

func recordMallWeatherProviderRequest(
	recorder mallWeatherMetricRecorder,
	endpoint string,
	success bool,
) {
	if recorder == nil || endpoint == "" {
		return
	}
	status := mallWeatherMetricStatusFailed
	if success {
		status = mallWeatherMetricStatusSuccess
	}
	recorder.AddCounter(
		MallWeatherMetricProviderRequestsTotal,
		map[string]string{"endpoint": endpoint, "status": status},
		1,
	)
}
