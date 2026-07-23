package data_svc

// MallWeatherMetricDefinition describes a registry-agnostic metric contract.
// A Prometheus, OpenTelemetry, or logging-backed exporter should use these
// definitions as the canonical mall weather metric names and label sets.
type MallWeatherMetricDefinition struct {
	Name   string
	Labels []string
}

var mallWeatherMetricDefinitions = []MallWeatherMetricDefinition{
	{Name: "mall_weather_fetch_total", Labels: []string{"kind", "status"}},
	{Name: "mall_weather_fetch_duration_seconds", Labels: []string{"kind"}},
	{Name: "mall_weather_provider_requests_total", Labels: []string{"endpoint", "status"}},
	{Name: "mall_weather_provider_rate_limited_total"},
	{Name: "mall_weather_provider_circuit_open"},
	{Name: "mall_weather_data_age_seconds", Labels: []string{"kind"}},
	{Name: "mall_weather_parse_warnings_total", Labels: []string{"field"}},
	{Name: "mall_weather_queue_lag_seconds", Labels: []string{"kind"}},
	{Name: "mall_weather_export_rows_total"},
	{Name: "mall_weather_feishu_rows_total", Labels: []string{"status"}},
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
