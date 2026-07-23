package data_svc

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"regexp"
	"sync"
	"testing"
	"time"

	"gin-biz-web-api/connector/caiyun"
	"gin-biz-web-api/job"
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/providerhttp"
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

func TestInMemoryMallWeatherMetricRecorderStoresGauges(t *testing.T) {
	t.Parallel()

	recorder := newInMemoryMallWeatherMetricRecorder()
	labels := map[string]string{"kind": "full"}
	recorder.SetGauge(MallWeatherMetricFetchDurationSeconds, labels, 1.25)
	labels["kind"] = "mutated"
	recorder.SetGauge(MallWeatherMetricFetchDurationSeconds, map[string]string{"kind": "full"}, 2.5)
	recorder.SetGauge(MallWeatherMetricDataAgeSeconds, nil, 10)
	recorder.SetGauge("", nil, 99)
	recorder.SetGauge(MallWeatherMetricDataAgeSeconds, map[string]string{"kind": "bad"}, math.NaN())
	recorder.SetGauge(MallWeatherMetricDataAgeSeconds, map[string]string{"kind": "bad"}, math.Inf(1))

	got := recorder.GaugeSnapshot()
	want := []MallWeatherMetricGaugeSample{
		{
			Name:   MallWeatherMetricDataAgeSeconds,
			Value:  10,
			Labels: nil,
		},
		{
			Name:   MallWeatherMetricFetchDurationSeconds,
			Labels: map[string]string{"kind": "full"},
			Value:  2.5,
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GaugeSnapshot()=%+v want %+v", got, want)
	}

	got[1].Labels["kind"] = "mutated"
	fresh := recorder.GaugeSnapshot()
	if fresh[1].Labels["kind"] != "full" {
		t.Fatalf("GaugeSnapshot() exposed mutable labels: %+v", fresh[1])
	}
}

func TestMallWeatherFetchGaugeRecorders(t *testing.T) {
	t.Parallel()

	recorder := newInMemoryMallWeatherMetricRecorder()
	startedAt := time.Date(2026, 7, 22, 3, 0, 0, 0, time.UTC)
	finishedAt := startedAt.Add(1500 * time.Millisecond)
	serverTime := startedAt.Add(-10 * time.Second)

	recordMallWeatherFetchDuration(recorder, "full", startedAt, finishedAt)
	recordMallWeatherDataAge(recorder, "full", &serverTime, finishedAt)
	recordMallWeatherFetchDuration(recorder, "bad", finishedAt, startedAt)
	futureServerTime := finishedAt.Add(time.Second)
	recordMallWeatherDataAge(recorder, "bad", &futureServerTime, finishedAt)

	got := recorder.GaugeSnapshot()
	want := []MallWeatherMetricGaugeSample{
		{
			Name:   MallWeatherMetricDataAgeSeconds,
			Labels: map[string]string{"kind": "full"},
			Value:  11.5,
		},
		{
			Name:   MallWeatherMetricFetchDurationSeconds,
			Labels: map[string]string{"kind": "full"},
			Value:  1.5,
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GaugeSnapshot()=%+v want %+v", got, want)
	}
}

func TestRecordMallWeatherProviderRateLimited(t *testing.T) {
	t.Parallel()

	recorder := newInMemoryMallWeatherMetricRecorder()

	recordMallWeatherProviderRateLimited(recorder, &caiyun.ProviderError{Class: providerhttp.ErrorClassRateLimited})
	recordMallWeatherProviderRateLimited(recorder, &caiyun.ProviderError{Class: providerhttp.ErrorClassProvider})
	recordMallWeatherProviderRateLimited(recorder, errors.New("redis limiter failed"))
	recordMallWeatherProviderRateLimited(recorder, nil)

	got := recorder.CounterSnapshot()
	want := []MallWeatherMetricCounterSample{{
		Name:  MallWeatherMetricProviderRateLimitedTotal,
		Value: 1,
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CounterSnapshot()=%+v want %+v", got, want)
	}
}

func TestRecordMallWeatherQueueLag(t *testing.T) {
	t.Parallel()

	recorder := newInMemoryMallWeatherMetricRecorder()
	availableAt := time.Date(2026, 7, 22, 3, 0, 0, 0, time.UTC)
	publishedAt := availableAt.Add(2500 * time.Millisecond)

	recordMallWeatherQueueLag(recorder, job.TypeMallWeatherFull, availableAt, publishedAt)
	recordMallWeatherQueueLag(recorder, job.TypeMallWeatherLifeIndex, availableAt, availableAt.Add(3*time.Second))
	recordMallWeatherQueueLag(recorder, "unknown", availableAt, publishedAt)
	recordMallWeatherQueueLag(recorder, job.TypeMallWeatherRepair, publishedAt, availableAt)
	recordMallWeatherQueueLag(recorder, job.TypeMallWeatherManual, time.Time{}, publishedAt)

	got := recorder.GaugeSnapshot()
	want := []MallWeatherMetricGaugeSample{
		{
			Name:   MallWeatherMetricQueueLagSeconds,
			Labels: map[string]string{"kind": "full"},
			Value:  2.5,
		},
		{
			Name:   MallWeatherMetricQueueLagSeconds,
			Labels: map[string]string{"kind": "lifeindex"},
			Value:  3,
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GaugeSnapshot()=%+v want %+v", got, want)
	}
}

func TestRecordMallWeatherOutboxQueueLag(t *testing.T) {
	recorder := newInMemoryMallWeatherMetricRecorder()
	previous := mallWeatherRuntimeMetrics
	mallWeatherRuntimeMetrics = recorder
	t.Cleanup(func() { mallWeatherRuntimeMetrics = previous })

	availableAt := time.Date(2026, 7, 22, 3, 0, 0, 0, time.UTC)
	RecordMallWeatherOutboxQueueLag(model.AsyncJobOutbox{
		TaskType:    job.TypeMallWeatherFeishu,
		AvailableAt: availableAt,
	}, availableAt.Add(4*time.Second))

	got := recorder.GaugeSnapshot()
	want := []MallWeatherMetricGaugeSample{
		{
			Name:   MallWeatherMetricQueueLagSeconds,
			Labels: map[string]string{"kind": "feishu"},
			Value:  4,
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GaugeSnapshot()=%+v want %+v", got, want)
	}
}

func TestMallWeatherParseWarningFieldIsLowCardinality(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "result module", path: "result.hourly.temperature[0].value", want: "hourly"},
		{name: "top level", path: "api_status", want: "api_status"},
		{name: "life index array", path: "data[12].lifeindex[3].type", want: "lifeindex"},
		{name: "empty", path: " ", want: "unknown"},
		{name: "sanitized", path: "result.bad-field.value", want: "bad_field"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := mallWeatherParseWarningField(test.path); got != test.want {
				t.Fatalf("mallWeatherParseWarningField(%q)=%q want %q", test.path, got, test.want)
			}
		})
	}
}

func TestRecordMallWeatherParseWarnings(t *testing.T) {
	t.Parallel()

	warningsJSON, err := json.Marshal([]caiyun.ParseWarning{
		{Code: "CORE_FIELD_COVERAGE_LOW", Path: "result.hourly.temperature"},
		{Code: "INVALID_ITEM", Path: "result.hourly.skycon[0]"},
		{Code: "API_STATUS_NOT_ACTIVE", Path: "api_status"},
		{Code: "EMPTY_DAY", Path: "data[0].lifeindex"},
	})
	if err != nil {
		t.Fatalf("json.Marshal() error=%v", err)
	}
	recorder := newInMemoryMallWeatherMetricRecorder()
	recordMallWeatherParseWarnings(recorder, model.JSONText(warningsJSON))
	recordMallWeatherParseWarnings(recorder, model.JSONText(`{"not":"a warning list"}`))

	got := recorder.CounterSnapshot()
	want := []MallWeatherMetricCounterSample{
		{
			Name:   MallWeatherMetricParseWarningsTotal,
			Labels: map[string]string{"field": "api_status"},
			Value:  1,
		},
		{
			Name:   MallWeatherMetricParseWarningsTotal,
			Labels: map[string]string{"field": "hourly"},
			Value:  2,
		},
		{
			Name:   MallWeatherMetricParseWarningsTotal,
			Labels: map[string]string{"field": "lifeindex"},
			Value:  1,
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CounterSnapshot()=%+v want %+v", got, want)
	}
}

func TestMallWeatherMetricsServiceSnapshotReturnsContractAndCounters(t *testing.T) {
	t.Parallel()

	recorder := newInMemoryMallWeatherMetricRecorder()
	recorder.AddCounter(MallWeatherMetricFeishuRowsTotal, map[string]string{"status": "success"}, 5)
	recorder.SetGauge(MallWeatherMetricDataAgeSeconds, map[string]string{"kind": "full"}, 12)
	service, err := newMallWeatherMetricsServiceWithRecorder(recorder)
	if err != nil {
		t.Fatalf("newMallWeatherMetricsServiceWithRecorder() error=%v", err)
	}

	result, err := service.Snapshot(context.Background(), 17)
	if err != nil {
		t.Fatalf("Snapshot() error=%v", err)
	}
	if len(result.Definitions) == 0 || len(result.Counters) != 1 || len(result.Gauges) != 1 ||
		result.Counters[0].Name != MallWeatherMetricFeishuRowsTotal ||
		result.Counters[0].Labels["status"] != "success" ||
		result.Counters[0].Value != 5 ||
		result.Gauges[0].Name != MallWeatherMetricDataAgeSeconds ||
		result.Gauges[0].Labels["kind"] != "full" ||
		result.Gauges[0].Value != 12 {
		t.Fatalf("Snapshot()=%+v", result)
	}

	result.Definitions[0].Name = "mutated"
	result.Counters[0].Labels["status"] = "mutated"
	result.Gauges[0].Labels["kind"] = "mutated"
	fresh, err := service.Snapshot(context.Background(), 17)
	if err != nil {
		t.Fatalf("Snapshot() second error=%v", err)
	}
	if fresh.Definitions[0].Name == "mutated" ||
		fresh.Counters[0].Labels["status"] != "success" ||
		fresh.Gauges[0].Labels["kind"] != "full" {
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
