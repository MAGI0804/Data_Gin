package data_svc

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
