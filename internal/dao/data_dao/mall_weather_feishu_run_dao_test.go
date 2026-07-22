package data_dao

import (
	"errors"
	"strings"
	"testing"
	"time"

	"gin-biz-web-api/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestClassifyMallWeatherFeishuRunStart(t *testing.T) {
	now := time.Date(2026, 7, 23, 8, 0, 0, 0, time.UTC)
	staleAfter := 10 * time.Minute
	tests := []struct {
		name   string
		mutate func(*MallWeatherFeishuRunRecord)
		want   MallWeatherFeishuRunDisposition
		err    bool
	}{
		{name: "pending acquired", want: MallWeatherFeishuRunDispositionAcquired},
		{
			name: "fresh running busy",
			mutate: func(record *MallWeatherFeishuRunRecord) {
				setFeishuRunLease(record, now.Add(-time.Minute))
			},
			want: MallWeatherFeishuRunDispositionBusy,
		},
		{
			name: "stale running recovered",
			mutate: func(record *MallWeatherFeishuRunRecord) {
				setFeishuRunLease(record, now.Add(-staleAfter))
			},
			want: MallWeatherFeishuRunDispositionAcquired,
		},
		{
			name: "success terminal",
			mutate: func(record *MallWeatherFeishuRunRecord) {
				setFeishuRunTerminal(record, "success", 3, 0, "", now.Add(-time.Minute))
			},
			want: MallWeatherFeishuRunDispositionTerminal,
		},
		{
			name: "partial terminal",
			mutate: func(record *MallWeatherFeishuRunRecord) {
				setFeishuRunTerminal(record, "partial_success", 2, 1, "one dataset failed", now.Add(-time.Minute))
			},
			want: MallWeatherFeishuRunDispositionTerminal,
		},
		{
			name: "failed terminal",
			mutate: func(record *MallWeatherFeishuRunRecord) {
				setFeishuRunTerminal(record, "failed", 0, 1, "push failed", now.Add(-time.Minute))
			},
			want: MallWeatherFeishuRunDispositionTerminal,
		},
		{
			name: "pending with token rejected",
			mutate: func(record *MallWeatherFeishuRunRecord) {
				record.Detail.RunToken = uuid.NewString()
			},
			err: true,
		},
		{
			name: "running without token rejected",
			mutate: func(record *MallWeatherFeishuRunRecord) {
				startedAt := model.TimeNormal{Time: now.Add(-time.Minute)}
				record.Pipeline.StartedAt = &startedAt
			},
			err: true,
		},
		{
			name: "unknown status rejected",
			mutate: func(record *MallWeatherFeishuRunRecord) {
				record.Pipeline.Status = "mystery"
			},
			err: true,
		},
		{
			name: "mismatched counts rejected",
			mutate: func(record *MallWeatherFeishuRunRecord) {
				record.Pipeline.TotalCount = 1
			},
			err: true,
		},
		{
			name: "clock rollback rejected",
			mutate: func(record *MallWeatherFeishuRunRecord) {
				record.Detail.UpdatedAt = now.Add(time.Minute)
			},
			err: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			record := storedFeishuRunRecord(now.Add(-time.Hour))
			if test.mutate != nil {
				test.mutate(&record)
			}
			got, err := classifyMallWeatherFeishuRunStart(&record, now, staleAfter)
			if test.err {
				if err == nil {
					t.Fatalf("classifyMallWeatherFeishuRunStart() got=%v, want error", got)
				}
				return
			}
			if err != nil || got != test.want {
				t.Fatalf("classifyMallWeatherFeishuRunStart() got=%v error=%v want=%v", got, err, test.want)
			}
		})
	}
}

func TestMallWeatherFeishuRunValidatesCreateAndFinish(t *testing.T) {
	record := newFeishuRunRecord()
	if !validNewMallWeatherFeishuRun(&record) {
		t.Fatal("validNewMallWeatherFeishuRun() rejected valid record")
	}
	invalid := record
	invalid.Detail.DestinationSnapshotJSON = model.JSONText(`[]`)
	if validNewMallWeatherFeishuRun(&invalid) {
		t.Fatal("validNewMallWeatherFeishuRun() accepted non-object snapshot")
	}
	invalid = record
	invalid.Detail.FiltersJSON = model.JSONText(`{"cities":["` + strings.Repeat("x", maxMallWeatherFeishuFiltersBytes) + `"]}`)
	if validNewMallWeatherFeishuRun(&invalid) {
		t.Fatal("validNewMallWeatherFeishuRun() accepted oversized filters")
	}
	now := time.Now().UTC()
	for _, finish := range []MallWeatherFeishuRunFinish{
		{Status: "success", SuccessCount: 0, FinishedAt: now},
		{Status: "failed", FailedCount: 1, SafeError: "push failed", FinishedAt: now},
		{Status: "partial_success", SuccessCount: 1, FailedCount: 1, SafeError: "one batch failed", FinishedAt: now},
	} {
		if !validMallWeatherFeishuRunFinish(finish) {
			t.Fatalf("validMallWeatherFeishuRunFinish() rejected %+v", finish)
		}
	}
	for _, finish := range []MallWeatherFeishuRunFinish{
		{Status: "success", FailedCount: 1, FinishedAt: now},
		{Status: "failed", FailedCount: 1, SafeError: "secret\nleak", FinishedAt: now},
		{Status: "partial_success", SuccessCount: 1, FinishedAt: now},
		{Status: "mystery", FinishedAt: now},
	} {
		if validMallWeatherFeishuRunFinish(finish) {
			t.Fatalf("validMallWeatherFeishuRunFinish() accepted %+v", finish)
		}
	}
}

func TestMallWeatherFeishuRunOwnedQueryBindsLeaseToken(t *testing.T) {
	token := uuid.NewString()
	db := dryRunWeatherDAOTestDB(t).Session(&gorm.Session{SkipDefaultTransaction: true})
	query := ownedMallWeatherFeishuPipelineQuery(db, 17, token).
		Update("updated_at", time.Now().UTC().Unix())
	if query.Error != nil {
		t.Fatalf("ownedMallWeatherFeishuPipelineQuery() error=%v", query.Error)
	}
	statement := query.Statement.SQL.String()
	for _, fragment := range []string{
		"id = ? AND status = ?",
		"EXISTS (SELECT 1 FROM mall_weather_feishu_runs",
		"pipeline_run_id = pipeline_runs.id AND run_token = ?",
	} {
		if !strings.Contains(statement, fragment) {
			t.Fatalf("owned query missing %q: %s", fragment, statement)
		}
	}
	if strings.Contains(statement, token) {
		t.Fatalf("owned query interpolates lease token: %s", statement)
	}
}

func TestMallWeatherFeishuRunRejectsInvalidDAOInputs(t *testing.T) {
	dao := &MallWeatherFeishuRunDAO{}
	if err := dao.Create(t.Context(), &MallWeatherFeishuRunRecord{}); err == nil {
		t.Fatal("Create() accepted unconfigured DAO")
	}
	if _, err := dao.FindByPipelineRunID(t.Context(), 1); err == nil {
		t.Fatal("FindByPipelineRunID() accepted unconfigured DAO")
	}
	if _, err := dao.BeginRun(t.Context(), 1, uuid.NewString(), time.Now(), time.Minute); err == nil {
		t.Fatal("BeginRun() accepted unconfigured DAO")
	}
	if err := dao.HeartbeatRun(t.Context(), 1, uuid.NewString(), time.Now()); err == nil {
		t.Fatal("HeartbeatRun() accepted unconfigured DAO")
	}
	if err := dao.UpdateRunProgress(t.Context(), 1, uuid.NewString(), MallWeatherFeishuRunProgress{
		UpdatedAt: time.Now(),
	}); err == nil {
		t.Fatal("UpdateRunProgress() accepted unconfigured DAO")
	}
	if err := dao.FinishRun(t.Context(), 1, uuid.NewString(), MallWeatherFeishuRunFinish{
		Status: "success", FinishedAt: time.Now(),
	}); err == nil {
		t.Fatal("FinishRun() accepted unconfigured DAO")
	}
	if !errors.Is(ErrMallWeatherFeishuRunLeaseLost, ErrMallWeatherFeishuRunLeaseLost) {
		t.Fatal("lease lost sentinel is not comparable")
	}
}

func newFeishuRunRecord() MallWeatherFeishuRunRecord {
	return MallWeatherFeishuRunRecord{
		Pipeline: model.PipelineRun{
			TraceID: uuid.NewString(), RunType: "delivery", TriggerType: "api",
			DestinationID: 9, Status: "running",
		},
		Detail: model.MallWeatherFeishuRun{
			ProfileID: 4, ProfileVersion: 2,
			ProfileSnapshotJSON:     model.JSONText(`{"profileId":4}`),
			FiltersJSON:             model.JSONText(`{"cities":["Shanghai"]}`),
			DestinationSnapshotJSON: model.JSONText(`{"writeMode":"append"}`),
			CreatedBy:               7,
		},
	}
}

func storedFeishuRunRecord(updatedAt time.Time) MallWeatherFeishuRunRecord {
	record := newFeishuRunRecord()
	record.Pipeline.ID = 11
	record.Pipeline.CreatedAt = int(updatedAt.Add(-time.Minute).Unix())
	record.Pipeline.UpdatedAt = int(updatedAt.Unix())
	record.Detail.ID = 12
	record.Detail.PipelineRunID = record.Pipeline.ID
	record.Detail.CreatedAt = updatedAt.Add(-time.Minute)
	record.Detail.UpdatedAt = updatedAt
	return record
}

func setFeishuRunLease(record *MallWeatherFeishuRunRecord, updatedAt time.Time) {
	startedAt := model.TimeNormal{Time: updatedAt.Add(-time.Minute)}
	record.Pipeline.StartedAt = &startedAt
	record.Pipeline.UpdatedAt = int(updatedAt.Unix())
	record.Detail.RunToken = uuid.NewString()
	record.Detail.UpdatedAt = updatedAt
}

func setFeishuRunTerminal(
	record *MallWeatherFeishuRunRecord,
	status string,
	successCount int,
	failedCount int,
	safeError string,
	finishedAt time.Time,
) {
	setFeishuRunLease(record, finishedAt.Add(-time.Minute))
	finished := model.TimeNormal{Time: finishedAt}
	record.Pipeline.Status = status
	record.Pipeline.SuccessCount = successCount
	record.Pipeline.FailedCount = failedCount
	record.Pipeline.TotalCount = successCount + failedCount
	record.Pipeline.ErrorMessage = safeError
	record.Pipeline.FinishedAt = &finished
}
