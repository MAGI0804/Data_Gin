package data_svc

import (
	"context"
	"testing"
	"time"

	"gin-biz-web-api/internal/reportrepo"
	"gin-biz-web-api/model"
)

func TestReportExportServiceCreatesOneFrozenExportPerRun(t *testing.T) {
	store := &fakeReportExportStore{}
	service := NewReportExportServiceWithStore(store)
	service.now = func() time.Time { return time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC) }
	result, replayed, err := service.Create(t.Context(), 17, 31)
	if err != nil || replayed || result.RunID != 31 || result.Status != model.ReportExportStatusPending ||
		store.actor != 17 || store.command.Outbox.TaskType != "report:export" || store.command.Outbox.TaskKey == "" {
		t.Fatalf("Create() = %#v replayed=%t error=%v store=%#v", result, replayed, err, store)
	}
	store.replayed = true
	result, replayed, err = service.Create(t.Context(), 17, 31)
	if err != nil || !replayed || result.ID != 41 {
		t.Fatalf("replayed Create() = %#v replayed=%t error=%v", result, replayed, err)
	}
}

type fakeReportExportStore struct {
	actor    uint
	replayed bool
	command  *reportrepo.CreateExportCommand
}

func (store *fakeReportExportStore) CreateOrGetExport(_ context.Context, actor, _ uint, command *reportrepo.CreateExportCommand) (bool, error) {
	store.actor, store.command = actor, command
	command.Export.ID = 41
	command.Export.CreatedAt = time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	return !store.replayed, nil
}
