package data_svc

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"gin-biz-web-api/connector/caiyun"
	weatherdomain "gin-biz-web-api/internal/weather"
	"gin-biz-web-api/job"
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/providerhttp"
)

// MallWeatherMetricDefinition describes a registry-agnostic metric contract.
// A Prometheus, OpenTelemetry, or logging-backed exporter should use these
// definitions as the canonical mall weather metric names and label sets.
type MallWeatherMetricDefinition struct {
	Name   string
	Labels []string
}

const (
	MallWeatherMetricFetchTotal                = "mall_weather_fetch_total"
	MallWeatherMetricFetchDurationSeconds      = "mall_weather_fetch_duration_seconds"
	MallWeatherMetricProviderRequestsTotal     = "mall_weather_provider_requests_total"
	MallWeatherMetricProviderRateLimitedTotal  = "mall_weather_provider_rate_limited_total"
	MallWeatherMetricProviderAuthFailuresTotal = "mall_weather_provider_auth_failures_total"
	MallWeatherMetricProviderCircuitOpen       = "mall_weather_provider_circuit_open"
	MallWeatherMetricDataAgeSeconds            = "mall_weather_data_age_seconds"
	MallWeatherMetricParseWarningsTotal        = "mall_weather_parse_warnings_total"
	MallWeatherMetricQueueLagSeconds           = "mall_weather_queue_lag_seconds"
	MallWeatherMetricDeadLettersTotal          = "mall_weather_dead_letters_total"
	MallWeatherMetricExportRowsTotal           = "mall_weather_export_rows_total"
	MallWeatherMetricFeishuRunsTotal           = "mall_weather_feishu_runs_total"
	MallWeatherMetricFeishuRowsTotal           = "mall_weather_feishu_rows_total"
	MallWeatherDeadLetterReasonInvalidPayload  = "invalid_payload"
	MallWeatherDeadLetterReasonPermanent       = "permanent_failure"
	mallWeatherMetricStatusSuccess             = "success"
	mallWeatherMetricStatusFailed              = "failed"
	mallWeatherMetricStatusPartialSuccess      = "partial_success"
	mallWeatherAlertStatusFiring               = "FIRING"
	mallWeatherAlertSeverityWarning            = "WARNING"
	mallWeatherAlertSeverityCritical           = "CRITICAL"
	mallWeatherFetchAlertMinSamples            = int64(20)
	mallWeatherProviderAlertMinSamples         = int64(20)
	mallWeatherFetchSuccessWarningRatio        = 0.95
	mallWeatherFetchSuccessCriticalRatio       = 0.80
	mallWeatherProviderRateLimitWarningRatio   = 0.05
	mallWeatherProviderRateLimitCriticalRatio  = 0.20
	mallWeatherQueueLagWarningSeconds          = 60
	mallWeatherQueueLagCriticalSeconds         = 300
)

var mallWeatherMetricDefinitions = []MallWeatherMetricDefinition{
	{Name: MallWeatherMetricFetchTotal, Labels: []string{"kind", "status"}},
	{Name: MallWeatherMetricFetchDurationSeconds, Labels: []string{"kind"}},
	{Name: MallWeatherMetricProviderRequestsTotal, Labels: []string{"endpoint", "status"}},
	{Name: MallWeatherMetricProviderRateLimitedTotal},
	{Name: MallWeatherMetricProviderAuthFailuresTotal, Labels: []string{"endpoint"}},
	{Name: MallWeatherMetricProviderCircuitOpen},
	{Name: MallWeatherMetricDataAgeSeconds, Labels: []string{"kind"}},
	{Name: MallWeatherMetricParseWarningsTotal, Labels: []string{"field"}},
	{Name: MallWeatherMetricQueueLagSeconds, Labels: []string{"kind"}},
	{Name: MallWeatherMetricDeadLettersTotal, Labels: []string{"kind", "reason"}},
	{Name: MallWeatherMetricExportRowsTotal},
	{Name: MallWeatherMetricFeishuRunsTotal, Labels: []string{"status"}},
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
	Alerts      []MallWeatherOperationalAlert    `json:"alerts"`
}

type MallWeatherOperationalAlert struct {
	Code      string            `json:"code"`
	Severity  string            `json:"severity"`
	Status    string            `json:"status"`
	Metric    string            `json:"metric"`
	Labels    map[string]string `json:"labels,omitempty"`
	Value     float64           `json:"value"`
	Threshold float64           `json:"threshold"`
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
	counters := service.metrics.CounterSnapshot()
	gauges := service.metrics.GaugeSnapshot()
	return &MallWeatherMetricsResult{
		Definitions: MallWeatherMetricDefinitions(),
		Counters:    counters,
		Gauges:      gauges,
		Alerts:      EvaluateMallWeatherOperationalAlerts(counters, gauges),
	}, nil
}

func EvaluateMallWeatherOperationalAlerts(
	counters []MallWeatherMetricCounterSample,
	gauges []MallWeatherMetricGaugeSample,
) []MallWeatherOperationalAlert {
	alerts := make([]MallWeatherOperationalAlert, 0)
	alerts = append(alerts, evaluateMallWeatherCounterAlerts(counters)...)
	alerts = append(alerts, evaluateMallWeatherGaugeAlerts(gauges)...)
	sort.Slice(alerts, func(left, right int) bool {
		leftKey := mallWeatherMetricSeriesKey(alerts[left].Code, alerts[left].Labels)
		rightKey := mallWeatherMetricSeriesKey(alerts[right].Code, alerts[right].Labels)
		return leftKey < rightKey
	})
	return alerts
}

func evaluateMallWeatherCounterAlerts(counters []MallWeatherMetricCounterSample) []MallWeatherOperationalAlert {
	var fetchTotal, fetchFailed, providerTotal, providerRateLimited int64
	alerts := make([]MallWeatherOperationalAlert, 0)
	for _, counter := range counters {
		switch counter.Name {
		case MallWeatherMetricFetchTotal:
			fetchTotal += counter.Value
			if counter.Labels["status"] == mallWeatherMetricStatusFailed {
				fetchFailed += counter.Value
			}
		case MallWeatherMetricProviderRequestsTotal:
			providerTotal += counter.Value
		case MallWeatherMetricProviderRateLimitedTotal:
			providerRateLimited += counter.Value
		case MallWeatherMetricProviderAuthFailuresTotal:
			if counter.Value > 0 {
				alerts = append(alerts, mallWeatherOperationalAlert(
					"MALL_WEATHER_PROVIDER_AUTH_FAILURES_PRESENT",
					mallWeatherAlertSeverityCritical,
					counter.Name,
					counter.Labels,
					float64(counter.Value),
					1,
				))
			}
		case MallWeatherMetricParseWarningsTotal:
			if counter.Value > 0 {
				alerts = append(alerts, mallWeatherOperationalAlert(
					"MALL_WEATHER_PARSE_WARNINGS_PRESENT",
					mallWeatherAlertSeverityWarning,
					counter.Name,
					counter.Labels,
					float64(counter.Value),
					1,
				))
			}
		case MallWeatherMetricDeadLettersTotal:
			if counter.Value > 0 {
				alerts = append(alerts, mallWeatherOperationalAlert(
					"MALL_WEATHER_DEAD_LETTERS_PRESENT",
					mallWeatherAlertSeverityCritical,
					counter.Name,
					counter.Labels,
					float64(counter.Value),
					1,
				))
			}
		case MallWeatherMetricFeishuRunsTotal:
			switch counter.Labels["status"] {
			case mallWeatherMetricStatusFailed:
				alerts = append(alerts, mallWeatherOperationalAlert(
					"MALL_WEATHER_FEISHU_RUNS_FAILED",
					mallWeatherAlertSeverityCritical,
					counter.Name,
					counter.Labels,
					float64(counter.Value),
					1,
				))
			case mallWeatherMetricStatusPartialSuccess:
				alerts = append(alerts, mallWeatherOperationalAlert(
					"MALL_WEATHER_FEISHU_RUNS_PARTIAL_SUCCESS",
					mallWeatherAlertSeverityWarning,
					counter.Name,
					counter.Labels,
					float64(counter.Value),
					1,
				))
			}
		}
	}
	if fetchTotal >= mallWeatherFetchAlertMinSamples {
		successRatio := float64(fetchTotal-fetchFailed) / float64(fetchTotal)
		switch {
		case successRatio < mallWeatherFetchSuccessCriticalRatio:
			alerts = append(alerts, mallWeatherOperationalAlert("MALL_WEATHER_FETCH_SUCCESS_RATE_CRITICAL", mallWeatherAlertSeverityCritical, MallWeatherMetricFetchTotal, nil, successRatio, mallWeatherFetchSuccessCriticalRatio))
		case successRatio < mallWeatherFetchSuccessWarningRatio:
			alerts = append(alerts, mallWeatherOperationalAlert("MALL_WEATHER_FETCH_SUCCESS_RATE_LOW", mallWeatherAlertSeverityWarning, MallWeatherMetricFetchTotal, nil, successRatio, mallWeatherFetchSuccessWarningRatio))
		}
	}
	if providerTotal >= mallWeatherProviderAlertMinSamples {
		rateLimitRatio := float64(providerRateLimited) / float64(providerTotal)
		switch {
		case rateLimitRatio >= mallWeatherProviderRateLimitCriticalRatio:
			alerts = append(alerts, mallWeatherOperationalAlert("MALL_WEATHER_PROVIDER_RATE_LIMITED_CRITICAL", mallWeatherAlertSeverityCritical, MallWeatherMetricProviderRateLimitedTotal, nil, rateLimitRatio, mallWeatherProviderRateLimitCriticalRatio))
		case rateLimitRatio >= mallWeatherProviderRateLimitWarningRatio:
			alerts = append(alerts, mallWeatherOperationalAlert("MALL_WEATHER_PROVIDER_RATE_LIMITED_HIGH", mallWeatherAlertSeverityWarning, MallWeatherMetricProviderRateLimitedTotal, nil, rateLimitRatio, mallWeatherProviderRateLimitWarningRatio))
		}
	}
	return alerts
}

func evaluateMallWeatherGaugeAlerts(gauges []MallWeatherMetricGaugeSample) []MallWeatherOperationalAlert {
	alerts := make([]MallWeatherOperationalAlert, 0)
	for _, gauge := range gauges {
		switch gauge.Name {
		case MallWeatherMetricProviderCircuitOpen:
			if gauge.Value >= 1 {
				alerts = append(alerts, mallWeatherOperationalAlert("MALL_WEATHER_PROVIDER_CIRCUIT_OPEN", mallWeatherAlertSeverityCritical, gauge.Name, nil, gauge.Value, 1))
			}
		case MallWeatherMetricDataAgeSeconds:
			warning, critical, ok := mallWeatherDataAgeAlertThresholds(gauge.Labels["kind"])
			if !ok {
				continue
			}
			switch {
			case gauge.Value >= critical:
				alerts = append(alerts, mallWeatherOperationalAlert("MALL_WEATHER_DATA_AGE_CRITICAL", mallWeatherAlertSeverityCritical, gauge.Name, gauge.Labels, gauge.Value, critical))
			case gauge.Value >= warning:
				alerts = append(alerts, mallWeatherOperationalAlert("MALL_WEATHER_DATA_AGE_HIGH", mallWeatherAlertSeverityWarning, gauge.Name, gauge.Labels, gauge.Value, warning))
			}
		case MallWeatherMetricQueueLagSeconds:
			switch {
			case gauge.Value >= mallWeatherQueueLagCriticalSeconds:
				alerts = append(alerts, mallWeatherOperationalAlert("MALL_WEATHER_QUEUE_LAG_CRITICAL", mallWeatherAlertSeverityCritical, gauge.Name, gauge.Labels, gauge.Value, mallWeatherQueueLagCriticalSeconds))
			case gauge.Value >= mallWeatherQueueLagWarningSeconds:
				alerts = append(alerts, mallWeatherOperationalAlert("MALL_WEATHER_QUEUE_LAG_HIGH", mallWeatherAlertSeverityWarning, gauge.Name, gauge.Labels, gauge.Value, mallWeatherQueueLagWarningSeconds))
			}
		}
	}
	return alerts
}

func mallWeatherOperationalAlert(
	code string,
	severity string,
	metric string,
	labels map[string]string,
	value float64,
	threshold float64,
) MallWeatherOperationalAlert {
	return MallWeatherOperationalAlert{
		Code:      code,
		Severity:  severity,
		Status:    mallWeatherAlertStatusFiring,
		Metric:    metric,
		Labels:    copyMallWeatherMetricLabels(labels),
		Value:     value,
		Threshold: threshold,
	}
}

func mallWeatherDataAgeAlertThresholds(taskKind string) (float64, float64, bool) {
	dataKind := ""
	switch taskKind {
	case "fast":
		dataKind = model.MallWeatherDataKindRealtime
	case "full", "repair", "manual":
		dataKind = model.MallWeatherDataKindHourly
	case "lifeindex":
		dataKind = model.MallWeatherDataKindLife
	default:
		return 0, 0, false
	}
	thresholds, err := weatherdomain.FreshnessThresholdsForKind(dataKind)
	if err != nil {
		return 0, 0, false
	}
	return thresholds.Warning.Seconds(), thresholds.Critical.Seconds(), true
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

func recordMallWeatherFeishuRun(
	recorder mallWeatherMetricRecorder,
	status string,
) {
	if recorder == nil {
		return
	}
	switch status {
	case mallWeatherMetricStatusSuccess, mallWeatherMetricStatusFailed, mallWeatherMetricStatusPartialSuccess:
	default:
		status = "unknown"
	}
	recorder.AddCounter(
		MallWeatherMetricFeishuRunsTotal,
		map[string]string{"status": status},
		1,
	)
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

func recordMallWeatherProviderRateLimited(
	recorder mallWeatherMetricRecorder,
	err error,
) {
	if recorder == nil || err == nil {
		return
	}
	var providerError *caiyun.ProviderError
	if !errors.As(err, &providerError) || providerError.Class != providerhttp.ErrorClassRateLimited {
		return
	}
	recorder.AddCounter(MallWeatherMetricProviderRateLimitedTotal, nil, 1)
}

func recordMallWeatherProviderAuthFailure(
	recorder mallWeatherMetricRecorder,
	endpoint string,
	err error,
) {
	if recorder == nil || endpoint == "" || err == nil {
		return
	}
	var providerError *caiyun.ProviderError
	if !errors.As(err, &providerError) || providerError.Class != providerhttp.ErrorClassAuth {
		return
	}
	recorder.AddCounter(
		MallWeatherMetricProviderAuthFailuresTotal,
		map[string]string{"endpoint": endpoint},
		1,
	)
}

func recordMallWeatherProviderCircuitOpen(
	recorder mallWeatherMetricRecorder,
	open bool,
) {
	gauge, ok := recorder.(mallWeatherMetricGaugeRecorder)
	if !ok {
		return
	}
	value := 0.0
	if open {
		value = 1
	}
	gauge.SetGauge(MallWeatherMetricProviderCircuitOpen, nil, value)
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
	return mallWeatherTaskMetricKind(taskType)
}

func mallWeatherTaskMetricKind(taskType string) string {
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
	case job.TypeMallWeatherSchedule:
		return "schedule"
	default:
		return ""
	}
}

func RecordMallWeatherDeadLetterTask(taskType string, reason string) {
	recordMallWeatherDeadLetterTask(mallWeatherRuntimeMetrics, taskType, reason)
}

func recordMallWeatherDeadLetterTask(
	recorder mallWeatherMetricRecorder,
	taskType string,
	reason string,
) {
	if recorder == nil {
		return
	}
	kind := mallWeatherTaskMetricKind(taskType)
	if kind == "" {
		kind = "unknown"
	}
	reason = mallWeatherDeadLetterReason(reason)
	recorder.AddCounter(
		MallWeatherMetricDeadLettersTotal,
		map[string]string{"kind": kind, "reason": reason},
		1,
	)
}

func mallWeatherDeadLetterReason(reason string) string {
	switch reason {
	case MallWeatherDeadLetterReasonInvalidPayload, MallWeatherDeadLetterReasonPermanent:
		return reason
	default:
		return "unknown"
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
