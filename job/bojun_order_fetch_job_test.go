package job

import (
	"encoding/json"
	"testing"
)

func TestNewBojunOrderFetchTaskNormalizesLegacyModeToAPI(t *testing.T) {
	task, err := NewBojunOrderFetchTask(BojunOrderFetchPayload{
		StartTime: "2026-08-25 10:00:00", EndTime: "2026-08-25 11:00:00",
	})
	if err != nil {
		t.Fatalf("NewBojunOrderFetchTask() error = %v", err)
	}
	var payload BojunOrderFetchPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload.SourceMode != "api" {
		t.Fatalf("source mode = %q, want api", payload.SourceMode)
	}
}

func TestNewBojunOrderFetchTaskAcceptsOracleStatusTimeBackfill(t *testing.T) {
	task, err := NewBojunOrderFetchTask(BojunOrderFetchPayload{
		SourceMode: " ORACLE ", StartTime: "2026-08-25 10:00:00", EndTime: "2026-08-25 11:00:00",
	})
	if err != nil {
		t.Fatalf("NewBojunOrderFetchTask() error = %v", err)
	}
	var payload BojunOrderFetchPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload.SourceMode != "oracle" {
		t.Fatalf("source mode = %q, want oracle", payload.SourceMode)
	}
}

func TestNewBojunOrderFetchTaskRejectsMixedSourceMode(t *testing.T) {
	if _, err := NewBojunOrderFetchTask(BojunOrderFetchPayload{
		SourceMode: "both", StartTime: "2026-08-25 10:00:00", EndTime: "2026-08-25 11:00:00",
	}); err == nil {
		t.Fatal("NewBojunOrderFetchTask() accepted source_mode=both")
	}
}
