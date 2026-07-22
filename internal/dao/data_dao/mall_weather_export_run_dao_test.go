package data_dao

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"gin-biz-web-api/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestClassifyMallWeatherExportRunStart(t *testing.T) {
	now := time.Date(2026, 7, 22, 8, 0, 0, 0, time.UTC)
	startedAt := now.Add(-time.Hour)
	tests := []struct {
		name string
		job  model.MallWeatherExportJob
		want MallWeatherExportRunDisposition
		err  bool
	}{
		{name: "pending acquired", job: exportRunState("pending", now), want: MallWeatherExportRunDispositionAcquired},
		{
			name: "fresh running busy",
			job:  exportRunningState(startedAt, now.Add(-time.Minute)),
			want: MallWeatherExportRunDispositionBusy,
		},
		{
			name: "stale running recovered",
			job:  exportRunningState(startedAt, now.Add(-11*time.Minute)),
			want: MallWeatherExportRunDispositionAcquired,
		},
		{name: "succeeded terminal", job: exportRunState("succeeded", now), want: MallWeatherExportRunDispositionTerminal},
		{name: "failed terminal", job: exportRunState("failed", now), want: MallWeatherExportRunDispositionTerminal},
		{name: "cancelled terminal", job: exportRunState("cancelled", now), want: MallWeatherExportRunDispositionTerminal},
		{name: "expired terminal", job: exportRunState("expired", now), want: MallWeatherExportRunDispositionTerminal},
		{name: "unknown rejected", job: exportRunState("mystery", now), err: true},
		{name: "running without start rejected", job: exportRunState("running", now), err: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := classifyMallWeatherExportRunStart(&test.job, now, 10*time.Minute)
			if test.err {
				if err == nil {
					t.Fatalf("classifyMallWeatherExportRunStart() got=%v, want error", got)
				}
				return
			}
			if err != nil || got != test.want {
				t.Fatalf("classifyMallWeatherExportRunStart() got=%v error=%v want=%v", got, err, test.want)
			}
		})
	}
}

func TestMallWeatherExportRunCheckpointStrictRoundTrip(t *testing.T) {
	token := uuid.NewString()
	original := MallWeatherExportRunCheckpoint{
		RunToken: token, DatasetIndex: 2, SheetIndex: 3, RowsInSheet: 1000,
		Cursor: json.RawMessage(`{"forecastTime":"2026-07-22T08:00:00Z","id":17}`),
	}
	encoded, err := encodeMallWeatherExportRunCheckpoint(original)
	if err != nil {
		t.Fatalf("encodeMallWeatherExportRunCheckpoint() error=%v", err)
	}
	decoded, err := decodeMallWeatherExportRunCheckpoint(encoded)
	if err != nil {
		t.Fatalf("decodeMallWeatherExportRunCheckpoint() error=%v", err)
	}
	if decoded.RunToken != token || decoded.DatasetIndex != 2 || decoded.SheetIndex != 3 ||
		decoded.RowsInSheet != 1000 || string(decoded.Cursor) != string(original.Cursor) {
		t.Fatalf("decoded=%+v", decoded)
	}
	for _, invalid := range []model.JSONText{
		model.JSONText(`{"runToken":"` + token + `","unknown":true}`),
		model.JSONText(`{"runToken":"not-a-uuid"}`),
		model.JSONText(`{"runToken":"` + token + `"}{}`),
	} {
		if _, err := decodeMallWeatherExportRunCheckpoint(invalid); err == nil {
			t.Fatalf("decodeMallWeatherExportRunCheckpoint(%s) accepted invalid value", invalid)
		}
	}
}

func TestMallWeatherExportRunOwnershipQueryBindsToken(t *testing.T) {
	token := uuid.NewString()
	dao := NewMallWeatherExportJobDAO(dryRunWeatherDAOTestDB(t))
	dao.db = dao.db.Session(&gorm.Session{SkipDefaultTransaction: true})
	query := dao.ownedRunQuery(t.Context(), 17, token).
		Update("updated_at", time.Date(2026, 7, 22, 8, 0, 0, 0, time.UTC))
	if query.Error != nil {
		t.Fatalf("ownedRunQuery() error=%v", query.Error)
	}
	statement := query.Statement.SQL.String()
	if !strings.Contains(statement, "JSON_UNQUOTE(JSON_EXTRACT(last_cursor_json, '$.runToken')) = ?") ||
		strings.Contains(statement, token) {
		t.Fatalf("ownership statement does not bind token: %s", statement)
	}
}

func TestMallWeatherExportRunSuccessQueryRejectsCancellation(t *testing.T) {
	token := uuid.NewString()
	dao := NewMallWeatherExportJobDAO(dryRunWeatherDAOTestDB(t))
	dao.db = dao.db.Session(&gorm.Session{SkipDefaultTransaction: true})
	query := dao.ownedActiveRunQuery(t.Context(), 17, token).
		Update("updated_at", time.Date(2026, 7, 22, 8, 0, 0, 0, time.UTC))
	if query.Error != nil {
		t.Fatalf("ownedActiveRunQuery() error=%v", query.Error)
	}
	statement := query.Statement.SQL.String()
	if !strings.Contains(statement, "cancel_requested = ?") || strings.Contains(statement, token) {
		t.Fatalf("active ownership statement is not cancellation-safe: %s", statement)
	}
}

func TestMallWeatherExportRunTerminalInputValidation(t *testing.T) {
	dao := &MallWeatherExportJobDAO{}
	now := time.Now().UTC()
	token := uuid.NewString()
	if err := dao.MarkRunSucceeded(
		nil,
		1,
		token,
		"weather-exports/job/result.xlsx",
		strings.Repeat("a", 64),
		1024,
		now,
		now.Add(7*24*time.Hour),
	); err == nil {
		t.Fatal("MarkRunSucceeded() accepted unconfigured DAO")
	}
	if !validMallWeatherExportObjectKey("weather-exports/job/result.xlsx") ||
		validMallWeatherExportObjectKey("../secret.xlsx") || validMallWeatherExportObjectKey("/absolute.xlsx") ||
		validMallWeatherExportObjectKey(" weather-exports/job/result.xlsx") {
		t.Fatal("validMallWeatherExportObjectKey() returned an unsafe result")
	}
	if !validMallWeatherExportSafeError("export generation failed") ||
		validMallWeatherExportSafeError("secret\nleak") {
		t.Fatal("validMallWeatherExportSafeError() returned an unsafe result")
	}
}

func exportRunState(status string, updatedAt time.Time) model.MallWeatherExportJob {
	return model.MallWeatherExportJob{
		BaseModel:         model.BaseModel{ID: 17},
		Status:            status,
		WeatherTimestamps: model.WeatherTimestamps{CreatedAt: updatedAt.Add(-time.Hour), UpdatedAt: updatedAt},
	}
}

func exportRunningState(startedAt, updatedAt time.Time) model.MallWeatherExportJob {
	job := exportRunState("running", updatedAt)
	job.StartedAt = &startedAt
	return job
}
