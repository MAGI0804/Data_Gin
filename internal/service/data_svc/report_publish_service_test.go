package data_svc

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"gin-biz-web-api/internal/reportoracle"
	"gin-biz-web-api/internal/reportrepo"
	"gin-biz-web-api/model"
)

func TestReportPublishServicePublishesValidatedOracleContract(t *testing.T) {
	draft := publicationDraft()
	store := &fakePublicationStore{draft: draft, datasource: publicationDatasource()}
	inspector := &fakeReportOracleInspector{procedure: publicationProcedure(), columns: publicationResultColumns()}
	decryptor := &fakePublicationDecryptor{password: "oracle-password"}
	service := NewReportPublishService(store, decryptor, func(_ context.Context, config reportoracle.Config) (reportOracleInspector, error) {
		if config.Password != "oracle-password" || config.Host != "oracle.internal" || config.FetchArraySize != 800 {
			t.Fatalf("oracle config = %#v", config)
		}
		return inspector, nil
	})
	validatedAt := time.Date(2026, 8, 12, 11, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return validatedAt }

	result, err := service.Publish(t.Context(), 17, 9, 3)
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if store.publication.ContractHash == "" || len(store.publication.CompiledSpecJSON) == 0 || store.publication.SchemaValidatedAt != validatedAt {
		t.Fatalf("publication = %#v", store.publication)
	}
	if result.DefinitionID != 9 || result.VersionID != 23 || result.Version != 3 || len(result.ContractHash) != 64 || !inspector.closed {
		t.Fatalf("result = %#v closed=%t", result, inspector.closed)
	}
	if !result.PublishedAt.Equal(validatedAt.Add(time.Second)) {
		t.Fatalf("publishedAt = %v", result.PublishedAt)
	}
	if result.Validation == nil {
		t.Fatal("validation summary is nil")
	}
	if !result.Validation.ValidatedAt.Equal(validatedAt) || result.Validation.Procedure.ArgumentCount != 1 || result.Validation.Procedure.SignatureHash != store.publication.ProcedureSignatureHash {
		t.Fatalf("procedure validation = %#v", result.Validation.Procedure)
	}
	if result.Validation.Result.ColumnCount != 3 || result.Validation.Result.SchemaHash != store.publication.ResultSchemaHash || !result.Validation.Snapshot.UniqueKeyValidated {
		t.Fatalf("result validation = %#v snapshot=%#v", result.Validation.Result, result.Validation.Snapshot)
	}
	if result.Validation.Export.ExportableColumnCount != 1 || result.Validation.Export.SchemaHash != store.publication.ExportSchemaHash {
		t.Fatalf("export validation = %#v", result.Validation.Export)
	}
}

func TestReportPublishServicePublishesJSONInputResultTableContract(t *testing.T) {
	draft := publicationDraft()
	draft.Version.ExecutionMode = model.ReportExecutionModeTableSnapshot
	draft.Version.JSONInputArgName = "P_PAYLOAD"
	draft.Version.InputSchemaJSON = model.JSONText(`{"store_id":{"type":"VARCHAR2","displayName":"门店"}}`)
	draft.Version.CallTemplate = "BEGIN REPORT.PKG_SALES.BUILD_REPORT(P_PAYLOAD => :payload); END;"
	draft.Parameters = nil
	procedure := []reportoracle.ProcedureArgument{{Name: "P_PAYLOAD", Position: 1, Sequence: 1, Direction: "IN", DataType: "CLOB"}}
	store := &fakePublicationStore{draft: draft, datasource: publicationDatasource()}
	inspector := &fakeReportOracleInspector{procedure: procedure, columns: publicationResultColumns()}
	service := NewReportPublishService(store, &fakePublicationDecryptor{password: "password"}, func(context.Context, reportoracle.Config) (reportOracleInspector, error) {
		return inspector, nil
	})

	result, err := service.Publish(t.Context(), 17, 9, 3)
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if result.Validation == nil || result.Validation.Procedure.ArgumentCount != 1 || result.Validation.Result.ColumnCount != 3 ||
		!strings.Contains(string(store.publication.CompiledSpecJSON), `"jsonInputArgName":"P_PAYLOAD"`) {
		t.Fatalf("result=%#v publication=%#v", result, store.publication)
	}
}

func TestReportPublishServiceDoesNotWriteWhenOracleValidationFails(t *testing.T) {
	store := &fakePublicationStore{draft: publicationDraft(), datasource: publicationDatasource()}
	inspector := &fakeReportOracleInspector{procedureErr: errors.New("oracle unavailable")}
	service := NewReportPublishService(store, &fakePublicationDecryptor{password: "password"}, func(context.Context, reportoracle.Config) (reportOracleInspector, error) { return inspector, nil })

	if _, err := service.Publish(t.Context(), 17, 9, 3); err == nil || !strings.Contains(err.Error(), "inspect Oracle procedure") {
		t.Fatalf("Publish() error = %v", err)
	}
	if store.publishCalls != 0 || !inspector.closed {
		t.Fatalf("publish calls = %d closed=%t", store.publishCalls, inspector.closed)
	}
}

func TestReportPublishServiceBoundsOracleInspectionAndClosesBeforePublishing(t *testing.T) {
	draft := publicationDraft()
	datasource := publicationDatasource()
	datasource.QueryTimeoutSeconds = 7
	store := &fakePublicationStore{draft: draft, datasource: datasource}
	inspector := &fakeReportOracleInspector{procedure: publicationProcedure(), columns: publicationResultColumns()}
	service := NewReportPublishService(store, &fakePublicationDecryptor{password: "password"}, func(ctx context.Context, _ reportoracle.Config) (reportOracleInspector, error) {
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > 7*time.Second || time.Until(deadline) < 6*time.Second {
			t.Fatalf("inspection deadline = %v, ok=%t", deadline, ok)
		}
		inspector.onInspect = func(ctx context.Context) {
			if _, ok := ctx.Deadline(); !ok {
				t.Fatal("inspection call is missing deadline")
			}
		}
		store.beforePublish = func() {
			if !inspector.closed {
				t.Fatal("publication persisted before Oracle inspector closed")
			}
		}
		return inspector, nil
	})

	if _, err := service.Publish(t.Context(), 17, 9, 3); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
}

func TestReportOracleQueryContextUsesDatasourceLimit(t *testing.T) {
	started := time.Now()
	ctx, cancel := reportOracleQueryContext(t.Context(), model.ReportDatasource{QueryTimeoutSeconds: 7})
	defer cancel()
	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("reportOracleQueryContext() did not set a deadline")
	}
	remaining := deadline.Sub(started)
	if remaining < 6*time.Second || remaining > 8*time.Second {
		t.Fatalf("deadline remaining = %v, want about 7s", remaining)
	}
}

func TestReportPublishServiceDoesNotWriteWhenOracleCloseFails(t *testing.T) {
	store := &fakePublicationStore{draft: publicationDraft(), datasource: publicationDatasource()}
	inspector := &fakeReportOracleInspector{procedure: publicationProcedure(), columns: publicationResultColumns(), closeErr: errors.New("close failed")}
	service := NewReportPublishService(store, &fakePublicationDecryptor{password: "password"}, func(context.Context, reportoracle.Config) (reportOracleInspector, error) {
		return inspector, nil
	})

	if _, err := service.Publish(t.Context(), 17, 9, 3); err == nil || !strings.Contains(err.Error(), "close oracle datasource") {
		t.Fatalf("Publish() error = %v", err)
	}
	if store.publishCalls != 0 || !inspector.closed {
		t.Fatalf("publish calls = %d closed=%t", store.publishCalls, inspector.closed)
	}
}

func TestReportPublishServiceRejectsStaleLockBeforeDecrypting(t *testing.T) {
	store := &fakePublicationStore{draft: publicationDraft(), datasource: publicationDatasource()}
	decryptor := &fakePublicationDecryptor{password: "password"}
	service := NewReportPublishService(store, decryptor, func(context.Context, reportoracle.Config) (reportOracleInspector, error) {
		return nil, errors.New("must not open")
	})

	if _, err := service.Publish(t.Context(), 17, 9, 2); !errors.Is(err, ErrReportConflict) {
		t.Fatalf("Publish() error = %v, want conflict", err)
	}
	if decryptor.calls != 0 || store.publishCalls != 0 {
		t.Fatalf("decrypt calls=%d publish calls=%d", decryptor.calls, store.publishCalls)
	}
}

func TestClassifyOracleInspectionErrorSeparatesContractAndInfrastructure(t *testing.T) {
	for _, source := range []error{reportoracle.ErrInvalidConfiguration, reportoracle.ErrMetadataMismatch, reportoracle.ErrUnsupportedBinding} {
		contractError := classifyOracleInspectionError("result snapshot", source)
		if !errors.Is(contractError, ErrReportPublicationInvalid) {
			t.Fatalf("contract error = %v", contractError)
		}
	}
	infrastructureError := classifyOracleInspectionError("result table", errors.New("connection reset"))
	if errors.Is(infrastructureError, ErrReportPublicationInvalid) {
		t.Fatalf("infrastructure error misclassified: %v", infrastructureError)
	}
}

func TestClassifyPublicationStoreErrorUsesResourceSpecificSemantics(t *testing.T) {
	tests := []struct {
		name   string
		source error
		want   error
	}{
		{name: "report missing", source: reportrepo.ErrDraftNotFound, want: ErrReportNotFound},
		{name: "stale version", source: reportrepo.ErrDraftVersionConflict, want: ErrReportConflict},
		{name: "datasource unavailable", source: reportrepo.ErrDatasourceUnavailable, want: ErrReportPublicationInvalid},
		{name: "invalid contract", source: reportrepo.ErrInvalidDraft, want: ErrReportPublicationInvalid},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := classifyPublicationStoreError(test.source); !errors.Is(err, test.want) {
				t.Fatalf("classifyPublicationStoreError() = %v, want %v", err, test.want)
			}
		})
	}
}

type fakePublicationStore struct {
	draft         *reportrepo.Draft
	datasource    *model.ReportDatasource
	publication   reportrepo.Publication
	publishCalls  int
	beforePublish func()
}

func (store *fakePublicationStore) FindDraftByID(context.Context, uint, uint) (*reportrepo.Draft, error) {
	return store.draft, nil
}
func (store *fakePublicationStore) FindDatasource(context.Context, uint) (*model.ReportDatasource, error) {
	return store.datasource, nil
}
func (store *fakePublicationStore) PublishDraft(_ context.Context, _ uint, _ uint, _ uint64, publication reportrepo.Publication) (*reportrepo.Draft, error) {
	if store.beforePublish != nil {
		store.beforePublish()
	}
	store.publishCalls++
	store.publication = publication
	published := *store.draft
	published.Version.Status = model.ReportVersionStatusPublished
	publishedAt := publication.SchemaValidatedAt.Add(time.Second)
	published.Version.PublishedAt = &publishedAt
	return &published, nil
}

type fakePublicationDecryptor struct {
	password string
	calls    int
}

func (decryptor *fakePublicationDecryptor) Decrypt(_, _ string) (string, error) {
	decryptor.calls++
	return decryptor.password, nil
}

type fakeReportOracleInspector struct {
	procedure    []reportoracle.ProcedureArgument
	columns      []reportoracle.ResultColumn
	procedureErr error
	closed       bool
	closeErr     error
	onInspect    func(context.Context)
}

func (inspector *fakeReportOracleInspector) InspectProcedure(ctx context.Context, _ reportoracle.ProcedureRef) ([]reportoracle.ProcedureArgument, error) {
	if inspector.onInspect != nil {
		inspector.onInspect(ctx)
	}
	return inspector.procedure, inspector.procedureErr
}
func (inspector *fakeReportOracleInspector) InspectResultTable(context.Context, reportoracle.ResultTableRef) ([]reportoracle.ResultColumn, error) {
	return inspector.columns, nil
}
func (inspector *fakeReportOracleInspector) InspectResultSnapshotContract(_ context.Context, ref reportoracle.ResultSnapshotRef) (reportoracle.ResultSnapshotContract, error) {
	return reportoracle.CompileResultSnapshotContract(ref, inspector.columns, true)
}

func (inspector *fakeReportOracleInspector) ValidateJSONSnapshotStore(context.Context) error {
	return nil
}
func (inspector *fakeReportOracleInspector) Close() error {
	inspector.closed = true
	return inspector.closeErr
}

func publicationDraft() *reportrepo.Draft {
	return &reportrepo.Draft{
		Definition: model.ReportDefinition{BaseModel: model.BaseModel{ID: 9}, Code: "sales", DatasourceID: 4, OwnerUserID: 17},
		Version: model.ReportVersion{BaseModel: model.BaseModel{ID: 23}, DefinitionID: 9, DatasourceID: 4, VersionNumber: 3, Status: model.ReportVersionStatusDraft,
			ProcedureOwner: "REPORT", PackageName: "PKG_SALES", ProcedureName: "BUILD_REPORT", ResultTableOwner: "REPORT", ResultTableName: "SALES_RESULT",
			ResultRunIDColumn: "RUN_ID", ResultRowIDColumn: "ROW_NO", CallTemplate: "BEGIN REPORT.PKG_SALES.BUILD_REPORT(P_RUN_ID => {{runId}}); END;"},
		Parameters: []model.ReportParameter{{ParameterCode: "runId", Label: "运行编号", DisplayOrder: 1, ControlType: "TEXT", LogicalType: "string", Cardinality: "SINGLE", ProcedureArgName: "P_RUN_ID", Position: 1, Direction: "IN", OracleType: "VARCHAR2", Required: true, SystemInjected: true, NullPolicy: "TYPED_NULL"}},
		Columns:    []model.ReportColumn{{FieldID: "11111111-1111-4111-8111-111111111111", LogicalCode: "orderNo", DatabaseColumn: "ORDER_NO", SourceOracleType: "VARCHAR2", ValueType: "string", PreviewHeader: "订单号", ExcelHeader: "订单号", DisplayOrder: 1, ExportOrder: 1, PreviewVisible: true, ExportVisible: true, ExportAllowed: true}},
		Grants:     []model.ReportGrant{{SubjectType: "ROLE", SubjectID: 2, ActionsJSON: model.JSONText(`["QUERY","EXPORT"]`)}}, LockVersion: 3,
	}
}

func publicationDatasource() *model.ReportDatasource {
	return &model.ReportDatasource{BaseModel: model.BaseModel{ID: 4}, Driver: model.ReportDatasourceDriverOracle, Host: "oracle.internal", Port: 1521, ServiceName: "REPORT", Username: "report_user", PasswordCiphertext: "cipher", CredentialKeyVersion: "key-v1", Enabled: true, ConnectTimeoutSeconds: 5, QueryTimeoutSeconds: 300, MaxOpenConnections: 4, MaxIdleConnections: 2, PrefetchRows: 500, ArraySize: 800}
}

func publicationProcedure() []reportoracle.ProcedureArgument {
	length := int64(64)
	return []reportoracle.ProcedureArgument{{Name: "P_RUN_ID", Position: 1, Sequence: 1, Direction: "IN", DataType: "VARCHAR2", DataLength: &length}}
}

func publicationResultColumns() []reportoracle.ResultColumn {
	zero := int64(0)
	eighteen := int64(18)
	return []reportoracle.ResultColumn{{Name: "RUN_ID", Position: 1, DataType: "VARCHAR2", DataLength: 64}, {Name: "ROW_NO", Position: 2, DataType: "NUMBER", DataLength: 22, DataPrecision: &eighteen, DataScale: &zero}, {Name: "ORDER_NO", Position: 3, DataType: "VARCHAR2", DataLength: 128}}
}
