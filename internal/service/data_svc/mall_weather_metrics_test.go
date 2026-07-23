package data_svc

import (
	"context"
	"errors"
	"math"
	"reflect"
	"regexp"
	"sync"
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

func TestInMemoryMallWeatherMetricRecorderAggregatesCounters(t *testing.T) {
	t.Parallel()

	recorder := newInMemoryMallWeatherMetricRecorder()
	labels := map[string]string{"status": "success", "kind": "feishu"}
	recorder.AddCounter(MallWeatherMetricFeishuRowsTotal, labels, 3)
	labels["status"] = "mutated"
	recorder.AddCounter(MallWeatherMetricFeishuRowsTotal, map[string]string{"kind": "feishu", "status": "success"}, 4)
	recorder.AddCounter(MallWeatherMetricFeishuRowsTotal, map[string]string{"status": "failed"}, 2)
	recorder.AddCounter("", nil, 10)
	recorder.AddCounter(MallWeatherMetricFeishuRowsTotal, nil, 0)

	got := recorder.CounterSnapshot()
	want := []MallWeatherMetricCounterSample{
		{
			Name:   MallWeatherMetricFeishuRowsTotal,
			Labels: map[string]string{"kind": "feishu", "status": "success"},
			Value:  7,
		},
		{
			Name:   MallWeatherMetricFeishuRowsTotal,
			Labels: map[string]string{"status": "failed"},
			Value:  2,
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CounterSnapshot()=%+v want %+v", got, want)
	}

	got[0].Labels["status"] = "mutated"
	fresh := recorder.CounterSnapshot()
	if fresh[0].Labels["status"] != "success" {
		t.Fatalf("CounterSnapshot() exposed mutable labels: %+v", fresh[0])
	}
}

func TestInMemoryMallWeatherMetricRecorderIsRaceSafe(t *testing.T) {
	t.Parallel()

	recorder := newInMemoryMallWeatherMetricRecorder()
	var wait sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for index := 0; index < 100; index++ {
				recorder.AddCounter(MallWeatherMetricFeishuRowsTotal, map[string]string{"status": "success"}, 1)
			}
		}()
	}
	wait.Wait()

	got := recorder.CounterSnapshot()
	if len(got) != 1 || got[0].Value != 800 {
		t.Fatalf("CounterSnapshot()=%+v, want one success counter with value 800", got)
	}
}

func TestInMemoryMallWeatherMetricRecorderSaturatesOnOverflow(t *testing.T) {
	t.Parallel()

	recorder := newInMemoryMallWeatherMetricRecorder()
	recorder.AddCounter(MallWeatherMetricFeishuRowsTotal, nil, math.MaxInt64)
	recorder.AddCounter(MallWeatherMetricFeishuRowsTotal, nil, 1)

	got := recorder.CounterSnapshot()
	if len(got) != 1 || got[0].Value != math.MaxInt64 {
		t.Fatalf("CounterSnapshot()=%+v, want saturated MaxInt64", got)
	}
}

func TestMallWeatherMetricsServiceSnapshotReturnsContractAndCounters(t *testing.T) {
	t.Parallel()

	recorder := newInMemoryMallWeatherMetricRecorder()
	recorder.AddCounter(MallWeatherMetricFeishuRowsTotal, map[string]string{"status": "success"}, 5)
	service, err := newMallWeatherMetricsServiceWithRecorder(recorder)
	if err != nil {
		t.Fatalf("newMallWeatherMetricsServiceWithRecorder() error=%v", err)
	}

	result, err := service.Snapshot(context.Background(), 17)
	if err != nil {
		t.Fatalf("Snapshot() error=%v", err)
	}
	if len(result.Definitions) == 0 || len(result.Counters) != 1 ||
		result.Counters[0].Name != MallWeatherMetricFeishuRowsTotal ||
		result.Counters[0].Labels["status"] != "success" ||
		result.Counters[0].Value != 5 {
		t.Fatalf("Snapshot()=%+v", result)
	}

	result.Definitions[0].Name = "mutated"
	result.Counters[0].Labels["status"] = "mutated"
	fresh, err := service.Snapshot(context.Background(), 17)
	if err != nil {
		t.Fatalf("Snapshot() second error=%v", err)
	}
	if fresh.Definitions[0].Name == "mutated" || fresh.Counters[0].Labels["status"] != "success" {
		t.Fatalf("Snapshot() exposed mutable state: %+v", fresh)
	}
}

func TestMallWeatherMetricsServiceSnapshotRejectsInvalidBoundary(t *testing.T) {
	t.Parallel()

	service, err := newMallWeatherMetricsServiceWithRecorder(newInMemoryMallWeatherMetricRecorder())
	if err != nil {
		t.Fatalf("newMallWeatherMetricsServiceWithRecorder() error=%v", err)
	}
	if _, err := service.Snapshot(context.Background(), 0); !errors.Is(err, ErrMallForbidden) {
		t.Fatalf("Snapshot() error=%v, want ErrMallForbidden", err)
	}
	if _, err := service.Snapshot(nil, 17); !errors.Is(err, ErrMallWeatherInvalidQuery) {
		t.Fatalf("Snapshot() error=%v, want ErrMallWeatherInvalidQuery", err)
	}
	if _, err := newMallWeatherMetricsServiceWithRecorder(nil); !errors.Is(err, ErrMallWeatherInvalidQuery) {
		t.Fatalf("newMallWeatherMetricsServiceWithRecorder(nil) error=%v, want ErrMallWeatherInvalidQuery", err)
	}
}
