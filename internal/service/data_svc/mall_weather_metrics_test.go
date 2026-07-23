package data_svc

import (
	"reflect"
	"regexp"
	"testing"
)

func TestMallWeatherMetricDefinitionsMatchDocumentedContract(t *testing.T) {
	t.Parallel()

	definitions := MallWeatherMetricDefinitions()
	want := []MallWeatherMetricDefinition{
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
	if !reflect.DeepEqual(definitions, want) {
		t.Fatalf("MallWeatherMetricDefinitions()=%+v, want %+v", definitions, want)
	}

	namePattern := regexp.MustCompile(`^mall_weather_[a-z0-9_]+$`)
	seen := make(map[string]struct{}, len(definitions))
	for _, definition := range definitions {
		if !namePattern.MatchString(definition.Name) {
			t.Fatalf("metric name %q is not stable snake_case", definition.Name)
		}
		if _, exists := seen[definition.Name]; exists {
			t.Fatalf("duplicate metric name %q", definition.Name)
		}
		seen[definition.Name] = struct{}{}
		for _, label := range definition.Labels {
			if !namePattern.MatchString("mall_weather_" + label) {
				t.Fatalf("metric label %q is not stable snake_case", label)
			}
		}
	}
}

func TestMallWeatherMetricDefinitionsReturnsDefensiveCopy(t *testing.T) {
	t.Parallel()

	first := MallWeatherMetricDefinitions()
	if len(first) == 0 || len(first[0].Labels) == 0 {
		t.Fatal("metric test requires at least one labeled definition")
	}
	first[0].Name = "mutated"
	first[0].Labels[0] = "mutated"

	second := MallWeatherMetricDefinitions()
	if second[0].Name == "mutated" || second[0].Labels[0] == "mutated" {
		t.Fatalf("MallWeatherMetricDefinitions() returned mutable global state: %+v", second[0])
	}
}
