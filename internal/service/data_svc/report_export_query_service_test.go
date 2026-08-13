package data_svc

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"gin-biz-web-api/internal/reportrepo"
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/storage"
)

func TestReportExportQueryServiceScopesActorAndSignsVerifiedObject(t *testing.T) {
	now := time.Date(2026, 8, 13, 10, 0, 0, 0, time.UTC)
	expires := now.Add(time.Hour)
	store := &fakeReportExportQueryStore{row: &model.ReportExport{
		BaseModel: model.BaseModel{ID: 41}, ExportUUID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", RunID: 31,
		Status: model.ReportExportStatusReady, ResultObjectKey: "private/report.xlsx", ResultChecksum: "checksum",
		FileSizeBytes: 123, ReadyAt: &now, ExpiresAt: &expires, CreatedBy: 17,
	}}
	signer := &fakeReportExportSigner{size: 123}
	service := NewReportExportQueryServiceWithDependencies(store, func() (reportExportDownloadSigner, error) { return signer, nil })
	service.now = func() time.Time { return now }
	view, err := service.Get(t.Context(), 17, 41)
	if err != nil || !view.CanDownload || store.actor != 17 || view.FileSizeBytes != 123 {
		t.Fatalf("Get()=%#v error=%v store=%#v", view, err, store)
	}
	download, err := service.Download(t.Context(), 17, 41)
	if err != nil || download.URL != "https://download.example/report" || signer.objectKey != "private/report.xlsx" || signer.fileName == "" {
		t.Fatalf("Download()=%#v error=%v signer=%#v", download, err, signer)
	}
	if len(store.audits) != 1 || store.audits[0].Action != "REPORT_EXPORT_DOWNLOAD_SIGN_SUCCESS" ||
		store.audits[0].TargetType != "REPORT_EXPORT" || store.audits[0].TargetID != 41 ||
		strings.Contains(string(store.audits[0].DetailJSON), "download.example") {
		t.Fatalf("download audit = %#v", store.audits)
	}
}

func TestReportExportDownloadRequiresSuccessAuditAndPreservesDeniedError(t *testing.T) {
	now := time.Date(2026, 8, 13, 10, 0, 0, 0, time.UTC)
	expires := now.Add(time.Hour)
	store := &fakeReportExportQueryStore{row: &model.ReportExport{
		BaseModel: model.BaseModel{ID: 41}, ExportUUID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
		Status: model.ReportExportStatusReady, ResultObjectKey: "private/report.xlsx", ResultChecksum: "checksum",
		FileSizeBytes: 123, ReadyAt: &now, ExpiresAt: &expires, CreatedBy: 17,
	}, auditErr: errors.New("audit unavailable")}
	service := NewReportExportQueryServiceWithDependencies(store, func() (reportExportDownloadSigner, error) {
		return &fakeReportExportSigner{size: 123}, nil
	})
	service.now = func() time.Time { return now }
	if _, err := service.Download(t.Context(), 17, 41); !errors.Is(err, ErrReportExportStorageUnavailable) {
		t.Fatalf("success audit failure error = %v", err)
	}

	store.row.Status = model.ReportExportStatusRunning
	if _, err := service.Download(t.Context(), 17, 41); !errors.Is(err, ErrReportExportQueryNotReady) {
		t.Fatalf("denied audit failure error = %v", err)
	}
}

func TestReportExportQueryServiceListsMappedActorExports(t *testing.T) {
	now := time.Date(2026, 8, 13, 10, 0, 0, 0, time.UTC)
	expires, purgeStarted := now.Add(time.Hour), now.Add(-time.Minute)
	store := &fakeReportExportQueryStore{page: &reportrepo.ExportListPage{
		Items: []reportrepo.ExportListRecord{{
			Export: model.ReportExport{
				BaseModel: model.BaseModel{ID: 41}, RunID: 31, Status: model.ReportExportStatusReady,
				ExportedRows: 120, PurgedRows: 80, ReadyAt: &now, ExpiresAt: &expires, PurgeStartedAt: &purgeStarted,
				ResultObjectKey: "reports/export.xlsx", FileSizeBytes: 1024, ResultChecksum: "checksum",
			},
			ReportName: "门店销售日报",
		}},
		HasMore: true, NextAfterID: 41,
	}}
	service := NewReportExportQueryServiceWithDependencies(store, func() (reportExportDownloadSigner, error) { return &fakeReportExportSigner{}, nil })
	service.now = func() time.Time { return now }

	page, err := service.List(t.Context(), 17, 55, 20, " ready ")
	if err != nil || store.actor != 17 || store.query.AfterID != 55 || store.query.Limit != 20 || store.query.Status != model.ReportExportStatusReady {
		t.Fatalf("List()=%#v error=%v store=%#v", page, err, store)
	}
	if !page.HasMore || page.NextAfterID != 41 || len(page.Items) != 1 {
		t.Fatalf("List() page = %#v", page)
	}
	item := page.Items[0]
	if item.ReportName != "门店销售日报" || item.PurgedRows != 80 || item.PurgeStartedAt != &purgeStarted || !item.CanDownload {
		t.Fatalf("List() item = %#v", item)
	}
	if _, err := service.List(t.Context(), 17, 0, 20, "UNKNOWN"); !errors.Is(err, ErrReportExportQueryInvalid) {
		t.Fatalf("List() invalid status error = %v", err)
	}
}

func TestReportExportCanDownloadRequiresCompleteReadyArtifact(t *testing.T) {
	now := time.Date(2026, 8, 13, 10, 0, 0, 0, time.UTC)
	future := now.Add(time.Hour)
	ready := now.Add(-time.Minute)
	complete := model.ReportExport{
		Status: model.ReportExportStatusReady, ReadyAt: &ready, ExpiresAt: &future,
		ResultObjectKey: "reports/export.xlsx", FileSizeBytes: 1024, ResultChecksum: "checksum",
	}
	if !reportExportCanDownload(complete, now) {
		t.Fatal("complete ready export should be downloadable")
	}
	tests := []struct {
		name   string
		mutate func(*model.ReportExport)
	}{
		{name: "missing ready time", mutate: func(row *model.ReportExport) { row.ReadyAt = nil }},
		{name: "missing expiry", mutate: func(row *model.ReportExport) { row.ExpiresAt = nil }},
		{name: "expired at current time", mutate: func(row *model.ReportExport) { row.ExpiresAt = &now }},
		{name: "missing object", mutate: func(row *model.ReportExport) { row.ResultObjectKey = "" }},
		{name: "empty file", mutate: func(row *model.ReportExport) { row.FileSizeBytes = 0 }},
		{name: "missing checksum", mutate: func(row *model.ReportExport) { row.ResultChecksum = "" }},
		{name: "not ready status", mutate: func(row *model.ReportExport) { row.Status = model.ReportExportStatusRunning }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			row := complete
			test.mutate(&row)
			if reportExportCanDownload(row, now) {
				t.Fatalf("incomplete export is downloadable: %#v", row)
			}
		})
	}
}

type fakeReportExportQueryStore struct {
	row      *model.ReportExport
	page     *reportrepo.ExportListPage
	actor    uint
	query    reportrepo.ExportListQuery
	audits   []model.ReportAudit
	auditErr error
}

func (store *fakeReportExportQueryStore) ListExportsForActor(_ context.Context, actor uint, query reportrepo.ExportListQuery) (*reportrepo.ExportListPage, error) {
	store.actor, store.query = actor, query
	if store.page == nil {
		return &reportrepo.ExportListPage{}, nil
	}
	return store.page, nil
}

func (store *fakeReportExportQueryStore) FindExportForActor(_ context.Context, actor, _ uint) (*model.ReportExport, error) {
	store.actor = actor
	return store.row, nil
}

func (store *fakeReportExportQueryStore) WriteReportAudit(_ context.Context, audit model.ReportAudit) error {
	store.audits = append(store.audits, audit)
	return store.auditErr
}

type fakeReportExportSigner struct {
	size      int64
	objectKey string
	fileName  string
}

func (signer *fakeReportExportSigner) StatDownloadObject(_ context.Context, objectKey string) (storage.ObjectMetadata, error) {
	signer.objectKey = objectKey
	return storage.ObjectMetadata{Size: signer.size}, nil
}

func (signer *fakeReportExportSigner) PresignDownloadURL(_ context.Context, objectKey, fileName string, _ time.Duration) (string, error) {
	signer.objectKey, signer.fileName = objectKey, fileName
	return "https://download.example/report", nil
}
