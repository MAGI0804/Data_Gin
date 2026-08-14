package data_svc

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"gin-biz-web-api/internal/reportrepo"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/model"
)

func TestReportDraftServiceCreateNormalizesContractAndHidesSensitiveFields(t *testing.T) {
	store := &fakeReportDraftStore{}
	service := NewReportDraftServiceWithStore(store)
	request := validReportDraftRequest()
	request.Parameters = append(request.Parameters, requestbody.ReportParameterRequest{
		Code: "secret", Label: "密钥", DisplayOrder: 2, ControlType: "TEXT", LogicalType: "string",
		Cardinality: "SINGLE", ProcedureArgName: "p_secret", Position: 2, OracleType: "VARCHAR2",
		Required: true, Sensitive: true, NullPolicy: "TYPED_NULL",
	})
	request.CallTemplate = "BEGIN REPORT_OWNER.PKG_SALES.BUILD_REPORT(P_RUN_ID => {{runId}}, P_SECRET => {{secret}}); END;"

	result, err := service.Create(t.Context(), 17, request)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if store.created == nil || store.created.Definition.OwnerUserID != 17 || store.created.Definition.Code != "sales-report" {
		t.Fatalf("created draft = %#v", store.created)
	}
	if store.created.Version.ProcedureOwner != "REPORT_OWNER" || store.created.Version.ResultRunIDColumn != "" || store.created.Version.ResultRowIDColumn != "" {
		t.Fatalf("normalized version = %#v", store.created.Version)
	}
	if result.Parameters[1].DefaultValue != nil {
		t.Fatal("sensitive parameter default leaked in DTO")
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal DTO: %v", err)
	}
	for _, forbidden := range []string{"compiledSpec", "contractHash", "ownerUserId", "currentDraftVersionId"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("DTO leaked %q: %s", forbidden, encoded)
		}
	}
}

func TestReportDraftServiceRejectsInvalidContractsBeforeStore(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*requestbody.ReportDraftSaveRequest)
	}{
		{name: "expected version on create", mutate: func(request *requestbody.ReportDraftSaveRequest) {
			value := uint64(1)
			request.ExpectedLockVersion = &value
		}},
		{name: "malformed placeholder", mutate: func(request *requestbody.ReportDraftSaveRequest) { request.CallTemplate = "BEGIN {{missing}}; END;" }},
		{name: "invalid identifier", mutate: func(request *requestbody.ReportDraftSaveRequest) { request.Result.TableName = "RESULT;DROP" }},
		{name: "duplicate excel header", mutate: func(request *requestbody.ReportDraftSaveRequest) {
			request.Columns = append(request.Columns, request.Columns[0])
			request.Columns[1].FieldID = uuid.NewString()
			request.Columns[1].LogicalCode = "other"
		}},
		{name: "required nullable", mutate: func(request *requestbody.ReportDraftSaveRequest) {
			request.Parameters[0].Required = true
			request.Parameters[0].Nullable = true
		}},
		{name: "unknown logical type", mutate: func(request *requestbody.ReportDraftSaveRequest) { request.Parameters[0].LogicalType = "shell" }},
		{name: "incompatible parameter control", mutate: func(request *requestbody.ReportDraftSaveRequest) { request.Parameters[0].ControlType = "CHECKBOX" }},
		{name: "unbound boolean Oracle type", mutate: func(request *requestbody.ReportDraftSaveRequest) {
			request.Parameters[0].ControlType = "CHECKBOX"
			request.Parameters[0].LogicalType = "boolean"
			request.Parameters[0].OracleType = "NUMBER"
		}},
		{name: "incompatible parameter control", mutate: func(request *requestbody.ReportDraftSaveRequest) { request.Parameters[0].ControlType = "CHECKBOX" }},
		{name: "collection encoding on scalar", mutate: func(request *requestbody.ReportDraftSaveRequest) {
			request.Parameters[0].CollectionEncoding = "JSON_CLOB"
		}},
		{name: "no exportable column", mutate: func(request *requestbody.ReportDraftSaveRequest) { request.Columns[0].ExportAllowed = false }},
		{name: "invalid result precision", mutate: func(request *requestbody.ReportDraftSaveRequest) {
			precision := 39
			request.Columns[0].Precision = &precision
		}},
		{name: "invalid grant actions", mutate: func(request *requestbody.ReportDraftSaveRequest) {
			request.Grants[0].Actions = json.RawMessage(`["QUERY","QUERY"]`)
		}},
		{name: "unsupported grant action", mutate: func(request *requestbody.ReportDraftSaveRequest) {
			request.Grants[0].Actions = json.RawMessage(`["ADMIN"]`)
		}},
		{name: "sensitive plaintext default", mutate: func(request *requestbody.ReportDraftSaveRequest) {
			request.Parameters[0].Sensitive = true
			request.Parameters[0].DefaultValue = json.RawMessage(`"private"`)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &fakeReportDraftStore{}
			service := NewReportDraftServiceWithStore(store)
			request := validReportDraftRequest()
			test.mutate(&request)
			if _, err := service.Create(t.Context(), 17, request); !errors.Is(err, ErrReportInvalid) {
				t.Fatalf("Create() error = %v, want ErrReportInvalid", err)
			}
			if store.createCalls != 0 {
				t.Fatalf("store create calls = %d", store.createCalls)
			}
		})
	}
}

func TestReportDraftServiceIgnoresNonExportableFieldsInExcelUniqueness(t *testing.T) {
	request := validReportDraftRequest()
	request.Columns[0].ExportAllowed = false
	second := request.Columns[0]
	second.FieldID = uuid.NewString()
	second.LogicalCode = "storeName"
	second.DatabaseColumn = "store_name"
	second.DisplayOrder = 2
	second.ExportAllowed = true
	request.Columns = append(request.Columns, second)

	if _, err := reportDraftFromRequest(17, request); err != nil {
		t.Fatalf("reportDraftFromRequest() error = %v", err)
	}
}

func TestReportDraftServiceAcceptsEnumParameterContracts(t *testing.T) {
	request := validReportDraftRequest()
	request.Parameters = append(request.Parameters, requestbody.ReportParameterRequest{
		Code: "region", Label: "区域", DisplayOrder: 2, ControlType: "SELECT", LogicalType: "enum",
		Cardinality: "SINGLE", ProcedureArgName: "p_region", Position: 2, OracleType: "VARCHAR2",
		Required: true, NullPolicy: "TYPED_NULL", AllowedValues: json.RawMessage(`["NORTH","SOUTH"]`),
	})
	request.CallTemplate = "BEGIN REPORT_OWNER.PKG_SALES.BUILD_REPORT(P_RUN_ID => {{runId}}, P_REGION => {{region}}); END;"

	store := &fakeReportDraftStore{}
	if _, err := NewReportDraftServiceWithStore(store).Create(t.Context(), 17, request); err != nil {
		t.Fatalf("Create() enum contract error = %v", err)
	}
}

func TestReportDraftServiceAcceptsRefCursorInputSchemaWithDisplayName(t *testing.T) {
	request := validReportDraftRequest()
	request.ExecutionMode = model.ReportExecutionModeRefCursor
	request.Procedure.JSONInputArgName = "p_payload"
	request.Procedure.ResultCursorArgName = "p_result"
	request.InputSchema = json.RawMessage(`{
		"c_supplier_id":{"type":"list[str]","displayName":"供应商","control":"multi_select","required":true,"example":["a","b"]},
		"datein_begin":{"type":"str","displayName":"开始日期","control":"date","format":"YYYYMMDD","example":"20260504"}
	}`)
	request.Result = requestbody.ReportResultRequest{}
	request.CallTemplate = ""
	request.Parameters = nil
	request.Columns[0].SourceOracleType = ""
	request.Columns[0].ValueType = ""
	request.Columns[0].Filterable = true
	request.Columns[0].Sortable = true

	draft, err := reportDraftFromRequest(17, request)
	if err != nil {
		t.Fatalf("reportDraftFromRequest() error = %v", err)
	}
	if draft.Version.ExecutionMode != model.ReportExecutionModeRefCursor || draft.Version.JSONInputArgName != "P_PAYLOAD" || draft.Version.ResultCursorArgName != "P_RESULT" {
		t.Fatalf("REF CURSOR contract = %#v", draft.Version)
	}
	if strings.Contains(string(draft.Version.InputSchemaJSON), `"label"`) || !strings.Contains(string(draft.Version.InputSchemaJSON), `"displayName":"供应商"`) ||
		!strings.Contains(string(draft.Version.InputSchemaJSON), `"type":"list[str]"`) || !strings.Contains(string(draft.Version.InputSchemaJSON), `"format":"YYYYMMDD"`) {
		t.Fatalf("canonical input schema = %s", draft.Version.InputSchemaJSON)
	}
	if len(draft.Parameters) != 0 || draft.Version.CallTemplate != "BEGIN REPORT_OWNER.PKG_SALES.BUILD_REPORT(P_PAYLOAD => :payload, P_RESULT => :resultCursor); END;" {
		t.Fatalf("REF CURSOR draft = %#v", draft)
	}
	if draft.Columns[0].SourceOracleType != "VARCHAR2" || draft.Columns[0].ValueType != "string" || draft.Columns[0].Filterable || draft.Columns[0].Sortable {
		t.Fatalf("REF CURSOR column defaults = %#v", draft.Columns[0])
	}
}

func TestReportDraftServiceRejectsRefCursorConditionWithoutDisplayName(t *testing.T) {
	request := validReportDraftRequest()
	request.ExecutionMode = model.ReportExecutionModeRefCursor
	request.Procedure.JSONInputArgName = "p_payload"
	request.Procedure.ResultCursorArgName = "p_result"
	request.InputSchema = json.RawMessage(`{"store_id":{"type":"VARCHAR2"}}`)
	request.Result = requestbody.ReportResultRequest{}
	request.Parameters = nil

	if _, err := reportDraftFromRequest(17, request); !errors.Is(err, ErrReportInvalid) {
		t.Fatalf("reportDraftFromRequest() error = %v, want ErrReportInvalid", err)
	}
}

func TestCanonicalReportInputSchemaValidatesConfiguredValuesAndDatePrecision(t *testing.T) {
	tests := []struct {
		name   string
		schema string
	}{
		{name: "number default is string", schema: `{"amount":{"type":"number","displayName":"金额","default":"12.5"}}`},
		{name: "list item type", schema: `{"stores":{"type":"list[str]","displayName":"门店","example":["S001",2]}}`},
		{name: "allowed value type", schema: `{"enabled":{"type":"bool","displayName":"启用","allowedValues":[true,"false"]}}`},
		{name: "date includes seconds", schema: `{"day":{"type":"str","displayName":"日期","control":"DATE","format":"YYYYMMDD","default":"20260504123045"}}`},
		{name: "datetime only includes day", schema: `{"at":{"type":"str","displayName":"时间","control":"DATETIME","format":"YYYY-MM-DD HH:mm:ss","example":"2026-05-04"}}`},
		{name: "iso datetime includes timezone", schema: `{"at":{"type":"str","displayName":"时间","control":"DATETIME","format":"ISO8601","example":"2026-05-04T12:30:45+08:00"}}`},
		{name: "iso datetime includes fractional seconds", schema: `{"at":{"type":"str","displayName":"时间","control":"DATETIME","format":"ISO8601","example":"2026-05-04T12:30:45.123"}}`},
		{name: "default outside allowed values", schema: `{"store":{"type":"str","displayName":"门店","allowedValues":["S001"],"default":"S002"}}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := canonicalReportInputSchema(json.RawMessage(test.schema)); !errors.Is(err, ErrReportInvalid) {
				t.Fatalf("canonicalReportInputSchema() error = %v, want ErrReportInvalid", err)
			}
		})
	}
}

func TestCanonicalReportInputSchemaKeepsDateAndDateTimeAsFormattedStrings(t *testing.T) {
	schema := json.RawMessage(`{
		"day":{"type":"str","displayName":"日期","control":"DATE","format":"YYYY-MM-DD","default":"2026-05-04"},
		"at":{"type":"str","displayName":"时间","control":"DATETIME","format":"YYYYMMDDHHmmss","example":"20260504123045"},
		"iso":{"type":"str","displayName":"同步时间","control":"DATETIME","format":"ISO8601","example":"2026-05-04T12:30:45"}
	}`)
	canonical, err := canonicalReportInputSchema(schema)
	if err != nil {
		t.Fatalf("canonicalReportInputSchema() error = %v", err)
	}
	if !strings.Contains(string(canonical), `"control":"DATE"`) || !strings.Contains(string(canonical), `"format":"YYYYMMDDHHmmss"`) {
		t.Fatalf("canonical schema = %s", canonical)
	}
}

func TestReportDraftServiceAcceptsJSONInputResultTable(t *testing.T) {
	request := validReportDraftRequest()
	request.ExecutionMode = model.ReportExecutionModeTableSnapshot
	request.Procedure.JSONInputArgName = "p_payload"
	request.InputSchema = json.RawMessage(`{
		"c_store_id":{"type":"list[str]","displayName":"门店","required":true},
		"datein_begin":{"type":"str","displayName":"开始日期","control":"DATE","format":"YYYYMMDD"}
	}`)
	request.Parameters = nil
	request.CallTemplate = ""

	draft, err := reportDraftFromRequest(17, request)
	if err != nil {
		t.Fatalf("reportDraftFromRequest() error = %v", err)
	}
	if !isJSONTableSnapshot(draft.Version) || draft.Version.JSONInputArgName != "P_PAYLOAD" || draft.Version.ResultCursorArgName != "" {
		t.Fatalf("JSON result-table contract = %#v", draft.Version)
	}
	if draft.Version.ResultTableOwner != "REPORT_OWNER" || draft.Version.ResultTableName != "SALES_RESULT" ||
		draft.Version.ResultRunIDColumn != "" || draft.Version.ResultRowIDColumn != "" {
		t.Fatalf("result table contract = %#v", draft.Version)
	}
	if got, want := draft.Version.CallTemplate, "BEGIN REPORT_OWNER.PKG_SALES.BUILD_REPORT(P_PAYLOAD => :payload); END;"; got != want {
		t.Fatalf("call template = %q, want %q", got, want)
	}
	if len(draft.Parameters) != 0 || !strings.Contains(string(draft.Version.InputSchemaJSON), `"displayName":"门店"`) {
		t.Fatalf("JSON input draft = %#v", draft)
	}
}

func TestReportDraftServiceTreatsFormerResultKeyNamesAsBusinessColumns(t *testing.T) {
	for _, keyColumn := range []string{"RUN_ID", "ID"} {
		t.Run(keyColumn, func(t *testing.T) {
			request := validReportDraftRequest()
			request.Columns[0].DatabaseColumn = keyColumn

			draft, err := reportDraftFromRequest(17, request)
			if err != nil || draft.Columns[0].DatabaseColumn != keyColumn {
				t.Fatalf("reportDraftFromRequest() draft=%#v error=%v", draft, err)
			}
		})
	}
}

func TestReportDraftServiceRejectsMixedJSONAndLegacyParameters(t *testing.T) {
	request := validReportDraftRequest()
	request.ExecutionMode = model.ReportExecutionModeTableSnapshot
	request.Procedure.JSONInputArgName = "p_payload"
	request.InputSchema = json.RawMessage(`{"store_id":{"type":"VARCHAR2","displayName":"门店"}}`)

	if _, err := reportDraftFromRequest(17, request); !errors.Is(err, ErrReportInvalid) {
		t.Fatalf("reportDraftFromRequest() error = %v, want ErrReportInvalid", err)
	}
}

func TestReportDraftServiceUpdateRequiresLockAndMapsMissingBeforeConflict(t *testing.T) {
	request := validReportDraftRequest()
	store := &fakeReportDraftStore{findErr: reportrepo.ErrDraftNotFound}
	service := NewReportDraftServiceWithStore(store)
	if _, err := service.Update(t.Context(), 17, 8, request); !errors.Is(err, ErrReportInvalid) {
		t.Fatalf("Update() without lock error = %v", err)
	}
	lockVersion := uint64(3)
	request.ExpectedLockVersion = &lockVersion
	if _, err := service.Update(t.Context(), 17, 8, request); !errors.Is(err, ErrReportNotFound) {
		t.Fatalf("Update() missing error = %v", err)
	}
	if store.updateCalls != 0 {
		t.Fatalf("update calls = %d", store.updateCalls)
	}

	store.findErr = nil
	store.found = draftFromValidRequestForTest(t, request)
	store.updateErr = reportrepo.ErrDraftVersionConflict
	if _, err := service.Update(t.Context(), 17, 8, request); !errors.Is(err, ErrReportConflict) {
		t.Fatalf("Update() conflict error = %v", err)
	}
}

func TestReportDraftServiceReturnsPersistedCreateAndUpdateState(t *testing.T) {
	persistedAt := time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)
	store := &fakeReportDraftStore{}
	service := NewReportDraftServiceWithStore(store)
	created, err := service.Create(t.Context(), 17, validReportDraftRequest())
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
		t.Fatalf("Create() did not return persisted timestamps: %#v", created)
	}
	store.found.Definition.CreatedAt = persistedAt
	store.found.Definition.UpdatedAt = persistedAt
	store.found.Definition.Status = model.ReportDefinitionStatusActive
	store.found.LockVersion = 1
	lockVersion := uint64(1)
	request := validReportDraftRequest()
	request.ExpectedLockVersion = &lockVersion

	updated, err := service.Update(t.Context(), 17, created.ID, request)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Status != model.ReportDefinitionStatusActive || !updated.CreatedAt.Equal(persistedAt) || !updated.UpdatedAt.Equal(persistedAt) {
		t.Fatalf("Update() persisted state = %#v", updated)
	}
}

func TestReportDraftServiceListUsesBoundedActorScopedQuery(t *testing.T) {
	store := &fakeReportDraftStore{page: reportrepo.DraftPage{
		Items:   []reportrepo.DraftSummary{{Definition: model.ReportDefinition{BaseModel: model.BaseModel{ID: 9}, Code: "sales", OwnerUserID: 17}, LockVersion: 4, IsOwner: true}},
		HasMore: true, NextAfterID: 9,
	}}
	service := NewReportDraftServiceWithStore(store)
	result, err := service.List(t.Context(), 17, 2, 20, " finance ", " sales ")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if store.listOwner != 17 || store.listQuery.AfterID != 2 || store.listQuery.Limit != 20 || store.listQuery.Category != "finance" || store.listQuery.Search != "sales" {
		t.Fatalf("list scope/query = owner %d, %#v", store.listOwner, store.listQuery)
	}
	if len(result.Items) != 1 || result.Items[0].LockVersion != 4 || !result.HasMore || result.NextAfterID != 9 {
		t.Fatalf("List() = %#v", result)
	}
	if _, err := service.List(t.Context(), 17, 0, 101, "", ""); !errors.Is(err, ErrReportInvalid) {
		t.Fatalf("List() invalid limit error = %v", err)
	}
}

func TestReportDraftServiceListRedactsSharedConfigurationMetadata(t *testing.T) {
	store := &fakeReportDraftStore{page: reportrepo.DraftPage{Items: []reportrepo.DraftSummary{
		{
			Definition:  model.ReportDefinition{BaseModel: model.BaseModel{ID: 9}, Code: "owned", Name: "自有报表", DatasourceID: 3, Status: model.ReportDefinitionStatusActive},
			LockVersion: 4, IsOwner: true,
		},
		{
			Definition:  model.ReportDefinition{BaseModel: model.BaseModel{ID: 10}, Code: "shared", Name: "共享报表", DatasourceID: 8, Status: model.ReportDefinitionStatusActive},
			LockVersion: 7, IsOwner: false,
		},
	}}}

	result, err := NewReportDraftServiceWithStore(store).List(t.Context(), 17, 0, 20, "", "")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(result.Items) != 2 {
		t.Fatalf("List() items = %#v", result.Items)
	}
	if result.Items[0].DatasourceID != 3 || result.Items[0].LockVersion != 4 {
		t.Fatalf("owned report metadata = %#v", result.Items[0])
	}
	if !result.Items[0].IsOwner {
		t.Fatalf("owned report ownership = %#v", result.Items[0])
	}
	if result.Items[1].IsOwner || result.Items[1].Status != model.ReportDefinitionStatusActive || result.Items[1].DatasourceID != 0 || result.Items[1].LockVersion != 0 {
		t.Fatalf("shared report metadata = %#v", result.Items[1])
	}
}

func validReportDraftRequest() requestbody.ReportDraftSaveRequest {
	return requestbody.ReportDraftSaveRequest{
		Code: " Sales-Report ", Name: "销售报表", Category: "finance", DatasourceID: 3,
		Procedure:    requestbody.ReportProcedureRequest{Owner: "report_owner", Package: "pkg_sales", Name: "build_report"},
		Result:       requestbody.ReportResultRequest{TableOwner: "report_owner", TableName: "sales_result"},
		CallTemplate: "BEGIN REPORT_OWNER.PKG_SALES.BUILD_REPORT(P_RUN_ID => {{runId}}); END;",
		Parameters: []requestbody.ReportParameterRequest{{
			Code: "runId", Label: "运行编号", DisplayOrder: 1, ControlType: "TEXT", LogicalType: "string",
			Cardinality: "SINGLE", ProcedureArgName: "p_run_id", Position: 1, OracleType: "VARCHAR2",
			Required: true, Nullable: false, SystemInjected: true, NullPolicy: "TYPED_NULL",
		}},
		Columns: []requestbody.ReportColumnRequest{{
			FieldID: uuid.NewString(), LogicalCode: "orderNo", DatabaseColumn: "order_no", SourceOracleType: "VARCHAR2",
			ValueType: "string", PreviewHeader: "订单号", ExcelHeader: "订单号", PreviewVisible: true,
			ExportVisible: true, ExportAllowed: true, DisplayOrder: 1, ExportOrder: 1, ExcelWidth: 18,
		}},
		Grants: []requestbody.ReportGrantRequest{{SubjectType: "ROLE", SubjectID: 2, Actions: json.RawMessage(`["QUERY","EXPORT"]`)}},
	}
}

func TestReportDraftRejectsSensitiveSystemParameter(t *testing.T) {
	request := validReportDraftRequest()
	request.Parameters[0].Sensitive = true
	if _, err := reportDraftFromRequest(17, request); !errors.Is(err, ErrReportInvalid) {
		t.Fatalf("reportDraftFromRequest() error = %v", err)
	}
}

func draftFromValidRequestForTest(t *testing.T, request requestbody.ReportDraftSaveRequest) *reportrepo.Draft {
	t.Helper()
	draft, err := reportDraftFromRequest(17, request)
	if err != nil {
		t.Fatalf("reportDraftFromRequest() error = %v", err)
	}
	draft.Definition.ID = 8
	draft.LockVersion = 3
	return draft
}

type fakeReportDraftStore struct {
	created     *reportrepo.Draft
	found       *reportrepo.Draft
	page        reportrepo.DraftPage
	findErr     error
	updateErr   error
	createCalls int
	updateCalls int
	listOwner   uint
	listQuery   reportrepo.DraftListQuery
}

func (store *fakeReportDraftStore) CreateDraft(_ context.Context, _ uint, draft *reportrepo.Draft) error {
	store.createCalls++
	store.created = draft
	draft.Definition.ID = 11
	persistedAt := time.Date(2026, 8, 12, 9, 0, 0, 0, time.UTC)
	draft.Definition.CreatedAt = persistedAt
	draft.Definition.UpdatedAt = persistedAt
	draft.LockVersion = 1
	store.found = draft
	return nil
}

func (store *fakeReportDraftStore) FindDraftByID(_ context.Context, _, _ uint) (*reportrepo.Draft, error) {
	if store.findErr != nil {
		return nil, store.findErr
	}
	if store.found == nil {
		store.found = &reportrepo.Draft{}
	}
	return store.found, nil
}

func (store *fakeReportDraftStore) ListDrafts(_ context.Context, owner uint, query reportrepo.DraftListQuery) (reportrepo.DraftPage, error) {
	store.listOwner = owner
	store.listQuery = query
	return store.page, nil
}

func (store *fakeReportDraftStore) UpdateDraft(_ context.Context, _, _ uint, _ uint64, draft *reportrepo.Draft) error {
	store.updateCalls++
	if store.updateErr == nil {
		if store.found != nil {
			draft.Definition.Status = store.found.Definition.Status
			draft.Definition.CreatedAt = store.found.Definition.CreatedAt
			draft.Definition.UpdatedAt = store.found.Definition.UpdatedAt
		}
		draft.LockVersion = 4
		store.found = draft
	}
	return store.updateErr
}
