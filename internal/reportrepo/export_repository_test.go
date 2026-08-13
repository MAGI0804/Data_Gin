package reportrepo

import (
	"context"
	"errors"
	"testing"
	"time"

	"gin-biz-web-api/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestNewReportExportOutboxContainsOnlyPlaceholderID(t *testing.T) {
	exportUUID := uuid.NewString()
	outbox := NewReportExportOutbox(exportUUID, time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC))
	if !validReportExportOutbox(outbox, exportUUID) || string(outbox.PayloadJSON) != `{"export_id":0}` {
		t.Fatalf("outbox=%#v", outbox)
	}
	bad := outbox
	bad.PayloadJSON = `{"export_id":0,"password":"secret"}`
	if validReportExportOutbox(bad, exportUUID) {
		t.Fatal("validReportExportOutbox() accepted secret payload")
	}
}

func TestAuthorizeRunExportUsesLiveAuthorizationOnlyForLegacySnapshots(t *testing.T) {
	legacy, err := encodeRunPermissionSnapshot(17, ReportActionQuery, runAuthority{Source: "USER", Grants: []model.ReportGrant{{SubjectType: "USER", SubjectID: 17, ActionsJSON: model.JSONText(`["QUERY"]`)}}})
	if err != nil {
		t.Fatalf("encode legacy snapshot: %v", err)
	}
	capability, err := encodeRunPermissionCapabilities(17, runAuthority{Source: "USER"}, runAuthority{Source: "USER"}, true)
	if err != nil {
		t.Fatalf("encode capability snapshot: %v", err)
	}

	tests := []struct {
		name      string
		snapshot  model.JSONText
		liveError error
		wantError error
		wantCalls int
	}{
		{name: "legacy current grant", snapshot: legacy, wantCalls: 1},
		{name: "legacy revoked grant", snapshot: legacy, liveError: ErrReportActionDenied, wantError: ErrReportExportRunNotReady, wantCalls: 1},
		{name: "legacy disabled report", snapshot: legacy, liveError: ErrPublishedReportNotFound, wantError: ErrReportExportRunNotReady, wantCalls: 1},
		{name: "legacy authorization query failed", snapshot: legacy, liveError: gorm.ErrInvalidDB, wantError: gorm.ErrInvalidDB, wantCalls: 1},
		{name: "current capability snapshot", snapshot: capability, wantCalls: 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			repository := New(newDryRunDB(t))
			repository.loadPublished = func(_ context.Context, _ *gorm.DB, actor, definitionID uint, action string, lock bool) (*PublishedReport, error) {
				calls++
				if actor != 17 || definitionID != 9 || action != ReportActionExport || lock {
					t.Fatalf("live authorization scope = actor %d definition %d action %s lock %t", actor, definitionID, action, lock)
				}
				return &PublishedReport{}, test.liveError
			}
			run := &model.ReportRun{DefinitionID: 9, PermissionSnapshotJSON: test.snapshot}
			err := repository.authorizeRunExport(t.Context(), repository.db, 17, run)
			if !errors.Is(err, test.wantError) || calls != test.wantCalls {
				t.Fatalf("authorizeRunExport() error = %v calls = %d, want %v/%d", err, calls, test.wantError, test.wantCalls)
			}
		})
	}
}
