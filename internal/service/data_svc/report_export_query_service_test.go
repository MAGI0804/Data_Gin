package data_svc

import (
	"context"
	"testing"
	"time"

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
}

type fakeReportExportQueryStore struct {
	row   *model.ReportExport
	actor uint
}

func (store *fakeReportExportQueryStore) FindExportForActor(_ context.Context, actor, _ uint) (*model.ReportExport, error) {
	store.actor = actor
	return store.row, nil
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
