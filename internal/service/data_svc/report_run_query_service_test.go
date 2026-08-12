package data_svc

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"gin-biz-web-api/internal/reportoracle"
	"gin-biz-web-api/internal/reportrepo"
	"gin-biz-web-api/model"
)

func TestReportRunQueryServiceScopesStatusAndCancellation(t *testing.T) {
	now := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	store := &fakeReportRunQueryStore{run: resultQueryRun(now)}
	service := newTestReportRunQueryService(store, &fakeReportResultReader{})
	service.now = func() time.Time { return now }
	got, err := service.Get(t.Context(), 17, 31)
	if err != nil || got.ID != 31 || !got.ResultAvailable || got.CanCancel {
		t.Fatalf("Get() = %#v, %v", got, err)
	}
	store.run.Status = model.ReportRunStatusRunning
	store.run.ResultExpiresAt = nil
	got, err = service.Cancel(t.Context(), 17, 31)
	if err != nil || !got.CancelRequested || !store.run.CancelRequested || store.actor != 17 {
		t.Fatalf("Cancel() = %#v, %v store=%#v", got, err, store)
	}
}

func TestReportRunQueryServiceReadsFrozenPreviewColumnsAndSignedCursor(t *testing.T) {
	now := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	run := resultQueryRun(now)
	run.PresentationSnapshotJSON = model.JSONText(`[
{"fieldId":"11111111-1111-4111-8111-111111111111","logicalCode":"amount","databaseColumn":"AMOUNT","sourceOracleType":"NUMBER","nullable":false,"valueType":"decimal","previewHeader":"金额","excelHeader":"金额","displayOrder":1,"exportOrder":1,"previewVisible":true,"exportVisible":true,"filterable":true,"sortable":true,"exportAllowed":true,"allowedOperators":[],"format":{},"dictionaryVersion":{},"maskingPolicy":{},"excelWidth":16,"nullDisplay":"-"},
{"fieldId":"22222222-2222-4222-8222-222222222222","logicalCode":"secret","databaseColumn":"SECRET_VALUE","sourceOracleType":"VARCHAR2","nullable":true,"valueType":"string","previewHeader":"密文","excelHeader":"密文","displayOrder":2,"exportOrder":2,"previewVisible":false,"exportVisible":false,"filterable":false,"sortable":false,"exportAllowed":false,"allowedOperators":[],"format":{},"dictionaryVersion":{},"maskingPolicy":{},"excelWidth":12,"nullDisplay":""}
]`)
	store := &fakeReportRunQueryStore{run: run, contract: &reportrepo.RunResultContract{Run: run, Datasource: model.ReportDatasource{CredentialKeyVersion: "v1", PasswordCiphertext: "cipher"}}}
	reader := &fakeReportResultReader{page: reportoracle.ResultPage{Rows: []reportoracle.ResultRow{{RowID: 9, Values: []interface{}{json.Number("123.45")}}}, NextRowID: 9, HasNext: true}}
	service := newTestReportRunQueryService(store, reader)
	service.now = func() time.Time { return now }
	page, err := service.ReadResults(t.Context(), 17, 31, "", 20)
	if err != nil || len(page.Columns) != 1 || page.Columns[0].Code != "amount" || len(page.Rows) != 1 || page.Rows[0].Values["amount"] != "123.45" || page.Pagination.NextCursor == "" {
		t.Fatalf("ReadResults() = %#v, %v", page, err)
	}
	if len(reader.columns) != 1 || reader.columns[0] != "AMOUNT" {
		t.Fatalf("Oracle columns = %#v", reader.columns)
	}
	if _, err := service.ReadResults(t.Context(), 17, 31, page.Pagination.NextCursor+"x", 20); !errors.Is(err, ErrReportRunQueryInvalid) {
		t.Fatalf("tampered cursor error = %v", err)
	}
	if _, err := service.ReadResults(t.Context(), 17, 31, page.Pagination.NextCursor, 10); !errors.Is(err, ErrReportRunQueryInvalid) {
		t.Fatalf("changed page size error = %v", err)
	}
}

func TestFrozenPreviewColumnsRejectsUnknownSnapshotFields(t *testing.T) {
	if _, err := frozenPreviewColumns(model.JSONText(`[{
"fieldId":"x","logicalCode":"x","databaseColumn":"X","previewVisible":true,"secret":"leak"
}]`)); err == nil {
		t.Fatal("frozenPreviewColumns() accepted unknown field")
	}
}

func TestReportResultCursorSupportsAnyOracleIntegerRowID(t *testing.T) {
	service := newTestReportRunQueryService(&fakeReportRunQueryStore{}, &fakeReportResultReader{})
	run := resultQueryRun(time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC))
	for _, rowID := range []int64{-9, 0, 9} {
		encoded, err := service.encodeCursor(reportResultCursor{Version: 1, RunUUID: run.RunUUID, ContractHash: run.ContractHash, PageSize: 20, AfterRowID: rowID})
		if err != nil {
			t.Fatalf("encode cursor %d: %v", rowID, err)
		}
		decoded, err := service.decodeCursor(encoded, run, 20)
		if err != nil || decoded.AfterRowID != rowID {
			t.Fatalf("decode cursor %d = %#v, %v", rowID, decoded, err)
		}
	}
}

func newTestReportRunQueryService(store *fakeReportRunQueryStore, reader *fakeReportResultReader) *ReportRunQueryService {
	return NewReportRunQueryServiceWithDependencies(store, fakeReportCredentialDecryptor{}, reader, []byte("0123456789abcdef0123456789abcdef"))
}

func resultQueryRun(now time.Time) model.ReportRun {
	expires := now.Add(time.Hour)
	return model.ReportRun{BaseModel: model.BaseModel{ID: 31}, RunUUID: "11111111-1111-4111-8111-111111111111", DefinitionID: 9, VersionID: 23, RequestedBy: 17, Status: model.ReportRunStatusSucceeded, ContractHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", RowCount: 1, ResultExpiresAt: &expires, WeatherTimestamps: model.WeatherTimestamps{CreatedAt: now.Add(-time.Minute)}}
}

type fakeReportRunQueryStore struct {
	actor    uint
	run      model.ReportRun
	contract *reportrepo.RunResultContract
	err      error
}

func (store *fakeReportRunQueryStore) FindRunForActor(_ context.Context, actor, _ uint) (*model.ReportRun, error) {
	store.actor = actor
	copy := store.run
	return &copy, store.err
}
func (store *fakeReportRunQueryStore) RequestRunCancellation(_ context.Context, actor, _ uint, _ time.Time) (*model.ReportRun, error) {
	store.actor = actor
	store.run.CancelRequested = true
	copy := store.run
	return &copy, store.err
}
func (store *fakeReportRunQueryStore) LoadResultContractForActor(_ context.Context, actor, _ uint, _ time.Time) (*reportrepo.RunResultContract, error) {
	store.actor = actor
	return store.contract, store.err
}

type fakeReportResultReader struct {
	columns []string
	after   *int64
	page    reportoracle.ResultPage
	err     error
}

func (reader *fakeReportResultReader) Read(_ context.Context, _ reportrepo.RunResultContract, password string, columns []string, after *int64, _ int) (reportoracle.ResultPage, error) {
	if password != "password" {
		return reportoracle.ResultPage{}, errors.New("unexpected password")
	}
	reader.columns = append([]string(nil), columns...)
	reader.after = after
	return reader.page, reader.err
}
