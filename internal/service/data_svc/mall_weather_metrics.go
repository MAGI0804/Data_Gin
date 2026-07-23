package data_svc

import (
	"context"
	"encoding/json"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"gin-biz-web-api/connector/caiyun"
	"gin-biz-web-api/job"
	"gin-biz-web-api/model"
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

type MallWeatherMetricGaugeSample struct {
	Name   string            `json:"name"`
	Labels map[string]string `json:"labels,omitempty"`
	Value  float64           `json:"value"`
}

type MallWeatherMetricsResult struct {
	Definitions []MallWeatherMetricDefinition    `json:"definitions"`
	Counters    []MallWeatherMetricCounterSample `json:"counters"`
	Gauges      []MallWeatherMetricGaugeSample   `json:"gauges"`
}

type mallWeatherMetricSnapshotter interface {
	CounterSnapshot() []MallWeatherMetricCounterSample
	GaugeSnapshot() []MallWeatherMetricGaugeSample
}

type mallWeatherMetricGaugeRecorder interface {
	SetGauge(name string, labels map[string]string, value float64)
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
		Gauges:      service.metrics.GaugeSnapshot(),
	}, nil
}

type inMemoryMallWeatherMetricRecorder struct {
	mu       sync.RWMutex
	counters map[string]MallWeatherMetricCounterSample
	gauges   map[string]MallWeatherMetricGaugeSample
}

func newInMemoryMallWeatherMetricRecorder() *inMemoryMallWeatherMetricRecorder {
	return &inMemoryMallWeatherMetricRecorder{
		counters: make(map[string]MallWeatherMetricCounterSample),
		gauges:   make(map[string]MallWeatherMetricGaugeSample),
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

func (recorder *inMemoryMallWeatherMetricRecorder) SetGauge(
	name string,
	labels map[string]string,
	value float64,
) {
	if recorder == nil || name == "" || math.IsNaN(value) || math.IsInf(value, 0) {
		return
	}
	key := mallWeatherMetricSeriesKey(name, labels)
	sample := MallWeatherMetricGaugeSample{
		Name:   name,
		Labels: copyMallWeatherMetricLabels(labels),
		Value:  value,
	}

	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	if recorder.gauges == nil {
		recorder.gauges = make(map[string]MallWeatherMetricGaugeSample)
	}
	recorder.gauges[key] = sample
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

func (recorder *inMemoryMallWeatherMetricRecorder) GaugeSnapshot() []MallWeatherMetricGaugeSample {
	if recorder == nil {
		return nil
	}
	recorder.mu.RLock()
	defer recorder.mu.RUnlock()

	samples := make([]MallWeatherMetricGaugeSample, 0, len(recorder.gauges))
	for _, sample := range recorder.gauges {
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

func recordMallWeatherFetchDuration(
	recorder mallWeatherMetricRecorder,
	taskKind string,
	startedAt time.Time,
	finishedAt time.Time,
) {
	gauge, ok := recorder.(mallWeatherMetricGaugeRecorder)
	if !ok || taskKind == "" || startedAt.IsZero() || finishedAt.IsZero() || finishedAt.Before(startedAt) {
		return
	}
	gauge.SetGauge(
		MallWeatherMetricFetchDurationSeconds,
		map[string]string{"kind": taskKind},
		finishedAt.Sub(startedAt).Seconds(),
	)
}

func recordMallWeatherDataAge(
	recorder mallWeatherMetricRecorder,
	taskKind string,
	providerServerTime *time.Time,
	observedAt time.Time,
) {
	gauge, ok := recorder.(mallWeatherMetricGaugeRecorder)
	if !ok || taskKind == "" || providerServerTime == nil || providerServerTime.IsZero() ||
		observedAt.IsZero() || observedAt.Before(*providerServerTime) {
		return
	}
	gauge.SetGauge(
		MallWeatherMetricDataAgeSeconds,
		map[string]string{"kind": taskKind},
		observedAt.Sub(*providerServerTime).Seconds(),
	)
}

// RecordMallWeatherOutboxQueueLag records the time a ready outbox row waited
// before it was durably marked as published to Asynq.
func RecordMallWeatherOutboxQueueLag(row model.AsyncJobOutbox, publishedAt time.Time) {
	recordMallWeatherQueueLag(mallWeatherRuntimeMetrics, row.TaskType, row.AvailableAt, publishedAt)
}

func recordMallWeatherQueueLag(
	recorder mallWeatherMetricRecorder,
	taskType string,
	availableAt time.Time,
	publishedAt time.Time,
) {
	gauge, ok := recorder.(mallWeatherMetricGaugeRecorder)
	if !ok || taskType == "" || availableAt.IsZero() || publishedAt.IsZero() || publishedAt.Before(availableAt) {
		return
	}
	taskKind := mallWeatherQueueLagTaskKind(taskType)
	if taskKind == "" {
		return
	}
	gauge.SetGauge(
		MallWeatherMetricQueueLagSeconds,
		map[string]string{"kind": taskKind},
		publishedAt.Sub(availableAt).Seconds(),
	)
}

func mallWeatherQueueLagTaskKind(taskType string) string {
	switch taskType {
	case job.TypeMallGeocode:
		return "geocode"
	case job.TypeMallWeatherFast:
		return "fast"
	case job.TypeMallWeatherFull:
		return "full"
	case job.TypeMallWeatherLifeIndex:
		return "lifeindex"
	case job.TypeMallWeatherRepair:
		return "repair"
	case job.TypeMallWeatherManual:
		return "manual"
	case job.TypeMallWeatherExport:
		return "export"
	case job.TypeMallWeatherExportCleanup:
		return "export_cleanup"
	case job.TypeMallWeatherFeishu:
		return "feishu"
	default:
		return ""
	}
}

func recordMallWeatherParseWarnings(
	recorder mallWeatherMetricRecorder,
	warningsJSON model.JSONText,
) {
	if recorder == nil || len(warningsJSON) == 0 {
		return
	}
	var warnings []caiyun.ParseWarning
	if err := json.Unmarshal([]byte(warningsJSON), &warnings); err != nil {
		return
	}
	for _, warning := range warnings {
		field := mallWeatherParseWarningField(warning.Path)
		recorder.AddCounter(
			MallWeatherMetricParseWarningsTotal,
			map[string]string{"field": field},
			1,
		)
	}
}

func mallWeatherParseWarningField(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "unknown"
	}
	path = strings.TrimPrefix(path, "result.")
	if strings.HasPrefix(path, "data[") || path == "data" {
		return "lifeindex"
	}
	fieldEnd := len(path)
	for index, char := range path {
		if char == '.' || char == '[' {
			fieldEnd = index
			break
		}
	}
	field := strings.ToLower(strings.TrimSpace(path[:fieldEnd]))
	var builder strings.Builder
	for _, char := range field {
		switch {
		case char >= 'a' && char <= 'z':
			builder.WriteRune(char)
		case char >= '0' && char <= '9':
			builder.WriteRune(char)
		case char == '_':
			builder.WriteRune(char)
		case builder.Len() > 0:
			builder.WriteByte('_')
		}
	}
	field = strings.Trim(builder.String(), "_")
	if field == "" {
		return "unknown"
	}
	return field
}
