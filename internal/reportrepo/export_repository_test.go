package reportrepo

import (
	"testing"
	"time"

	"github.com/google/uuid"
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
