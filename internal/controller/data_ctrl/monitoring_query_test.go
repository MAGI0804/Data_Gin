package data_ctrl

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"
	"time"

	"gin-biz-web-api/model"
)

func TestMonitoringQueryParsing(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		fallback  int
		maximum   int
		want      int
		wantError bool
	}{
		{name: "empty uses fallback", fallback: 20, maximum: 100, want: 20},
		{name: "valid", value: "30", fallback: 20, maximum: 100, want: 30},
		{name: "zero rejected", value: "0", fallback: 20, maximum: 100, wantError: true},
		{name: "over maximum rejected", value: "101", fallback: 20, maximum: 100, wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseMonitoringPositiveInt(tt.value, tt.fallback, tt.maximum)
			if (err != nil) != tt.wantError || got != tt.want {
				t.Fatalf("parseMonitoringPositiveInt(%q) = %d, %v; want %d, error=%v", tt.value, got, err, tt.want, tt.wantError)
			}
		})
	}

	if monitoringQueryKeysAllowed(url.Values{"page": {"1"}, "unexpected": {"x"}}, "page") {
		t.Fatal("monitoringQueryKeysAllowed accepted an unknown filter")
	}
	start, err := parseMonitoringTime("2026-07-31T09:30")
	if err != nil || start == nil || start.Location() != monitoringQueryLocation {
		t.Fatalf("parseMonitoringTime() = %v, %v", start, err)
	}
	end := start.Add(-time.Minute)
	if monitoringTimeRangeValid(start, &end) {
		t.Fatal("monitoringTimeRangeValid accepted an inverted range")
	}
}

func TestMonitoringSafeResponsesDoNotExposeSecretsOrRawErrors(t *testing.T) {
	runs := safePipelineRuns([]model.PipelineRun{{
		BaseModel: model.BaseModel{ID: 7}, TraceID: "trace-7", RunType: "delivery", Status: "failed", ErrorMessage: "authorization=secret-run-error",
	}})
	if len(runs) != 1 || runs[0].TraceID != "trace-7" {
		t.Fatalf("safePipelineRuns() = %#v", runs)
	}
	runJSON, err := json.Marshal(runs)
	if err != nil || strings.Contains(string(runJSON), "secret-run-error") || strings.Contains(string(runJSON), "error_message") {
		t.Fatalf("safePipelineRuns serialized unsafe result: %s, %v", runJSON, err)
	}

	logs := safeDeliveryLogs([]model.DeliveryLog{{
		BaseModel: model.BaseModel{ID: 8}, RequestBody: `{"Authorization":"secret-request"}`, ResponseBody: `{"token":"secret-response"}`,
		ErrorMessage: "token=secret-log-error", ResponseSummary: "password=secret-summary",
	}})
	if len(logs) != 1 || logs[0].ErrorMessage != "第三方响应包含敏感信息，详情已隐藏。" || logs[0].ResponseSummary != "第三方响应包含敏感信息，详情已隐藏。" {
		t.Fatalf("safeDeliveryLogs() = %#v", logs)
	}
	logJSON, err := json.Marshal(logs)
	if err != nil || strings.Contains(string(logJSON), "secret-request") || strings.Contains(string(logJSON), "secret-response") || strings.Contains(string(logJSON), "secret-log-error") || strings.Contains(string(logJSON), "secret-summary") || strings.Contains(string(logJSON), "request_body") || strings.Contains(string(logJSON), "response_body") {
		t.Fatalf("safeDeliveryLogs serialized unsafe result: %s, %v", logJSON, err)
	}
}
