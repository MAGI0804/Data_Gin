package reportrepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"gin-biz-web-api/model"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestDefinitionScopeAlwaysFiltersOwnerAndDraftStatus(t *testing.T) {
	db := newDryRunDB(t)
	statement := definitionScope(db.WithContext(context.Background()), 17).
		Where("id = ?", 42).
		Find(&[]definitionRecord{})
	if statement.Error != nil {
		t.Fatalf("build scoped query: %v", statement.Error)
	}
	sqlText := statement.Statement.SQL.String()
	for _, fragment := range []string{"owner_user_id = ?", "status IN", "id = ?"} {
		if !strings.Contains(sqlText, fragment) {
			t.Fatalf("scope SQL %q does not contain %q", sqlText, fragment)
		}
	}
	if len(statement.Statement.Vars) < 2 || statement.Statement.Vars[0] != uint(17) {
		t.Fatalf("scope vars = %#v", statement.Statement.Vars)
	}
}

func TestVersionScopeAlwaysFiltersDraftStatus(t *testing.T) {
	db := newDryRunDB(t)
	statement := versionScope(db.WithContext(context.Background())).
		Where("definition_id = ?", 9).
		Find(&[]versionRecord{})
	if statement.Error != nil {
		t.Fatalf("build scoped query: %v", statement.Error)
	}
	sqlText := statement.Statement.SQL.String()
	if !strings.Contains(sqlText, "status = ?") {
		t.Fatalf("scope SQL = %q", sqlText)
	}
}

func TestRepositoryRejectsUnscopedOrStaleInputBeforeDatabase(t *testing.T) {
	repository := &Repository{}
	if _, err := repository.FindDraftByID(context.Background(), 0, 1); !errors.Is(err, ErrInvalidDraft) {
		t.Fatalf("FindDraftByID() error = %v, want ErrInvalidDraft", err)
	}
	if _, err := repository.SaveDraftCollections(context.Background(), 1, 1, 1, nil, nil, nil); !errors.Is(err, ErrInvalidDraft) {
		t.Fatalf("SaveDraftCollections() error = %v, want ErrInvalidDraft", err)
	}

	db := newDryRunDB(t)
	repository = New(db)
	draft := validDraft()
	draft.LockVersion = 4
	if err := repository.UpdateDraft(context.Background(), 1, 7, 3, draft); !errors.Is(err, ErrInvalidDraft) {
		t.Fatalf("UpdateDraft() error = %v, want ErrInvalidDraft", err)
	}
}

func TestValidateCollectionsRejectsDuplicateStableKeys(t *testing.T) {
	tests := []struct {
		name       string
		parameters []model.ReportParameter
		columns    []model.ReportColumn
		grants     []model.ReportGrant
	}{
		{
			name: "parameter code",
			parameters: []model.ReportParameter{
				{ParameterCode: "runId", Position: 1},
				{ParameterCode: "runId", Position: 2},
			},
		},
		{
			name: "parameter position",
			parameters: []model.ReportParameter{
				{ParameterCode: "from", Position: 1},
				{ParameterCode: "to", Position: 1},
			},
		},
		{
			name: "column field id",
			columns: []model.ReportColumn{
				{LogicalCode: "a", FieldID: "field-1"},
				{LogicalCode: "b", FieldID: "field-1"},
			},
		},
		{
			name: "grant subject",
			grants: []model.ReportGrant{
				{SubjectType: "ROLE", SubjectID: 2},
				{SubjectType: "ROLE", SubjectID: 2},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := validateCollections(test.parameters, test.columns, test.grants); !errors.Is(err, ErrInvalidDraft) {
				t.Fatalf("validateCollections() error = %v, want ErrInvalidDraft", err)
			}
		})
	}
}

func TestValidateReferenceCountRejectsMissingOrDisabledReferences(t *testing.T) {
	tests := []struct {
		name      string
		reference string
		actual    int64
		expected  int
		wantError bool
	}{
		{name: "enabled datasource", reference: "datasource", actual: 1, expected: 1},
		{name: "missing or disabled datasource", reference: "datasource", actual: 0, expected: 1, wantError: true},
		{name: "all active users", reference: "grant user", actual: 2, expected: 2},
		{name: "one missing or disabled user", reference: "grant user", actual: 1, expected: 2, wantError: true},
		{name: "all active roles", reference: "grant role", actual: 2, expected: 2},
		{name: "one missing or disabled role", reference: "grant role", actual: 1, expected: 2, wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateReferenceCount(test.reference, test.actual, test.expected)
			if test.wantError && !errors.Is(err, ErrInvalidDraft) {
				t.Fatalf("validateReferenceCount() error = %v, want ErrInvalidDraft", err)
			}
			if !test.wantError && err != nil {
				t.Fatalf("validateReferenceCount() error = %v", err)
			}
		})
	}
}

func TestRepositoryReferenceValidationStopsDraftWrites(t *testing.T) {
	referenceError := invalidDraft("datasource or grant subject is unavailable")
	tests := []struct {
		name string
		run  func(*Repository) error
	}{
		{name: "create", run: func(repository *Repository) error {
			return repository.CreateDraft(context.Background(), 8, validDraft())
		}},
		{name: "update", run: func(repository *Repository) error {
			return repository.UpdateDraft(context.Background(), 8, 7, 1, validDraft())
		}},
		{name: "save collections", run: func(repository *Repository) error {
			_, err := repository.SaveDraftCollections(context.Background(), 8, 7, 1, nil, nil, nil)
			return err
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			referenceCalls := 0
			repository := New(newDryRunDB(t))
			repository.transact = func(_ context.Context, db *gorm.DB, operation func(*gorm.DB) error) error { return operation(db) }
			repository.lockDefinition = func(context.Context, *gorm.DB, uint, uint) (*definitionRecord, error) {
				return &definitionRecord{ReportDefinition: model.ReportDefinition{BaseModel: model.BaseModel{ID: 7}, DatasourceID: 3, CurrentDraftVersionID: 11}}, nil
			}
			repository.lockVersion = func(context.Context, *gorm.DB, uint, uint) (*versionRecord, error) {
				return &versionRecord{ReportVersion: model.ReportVersion{BaseModel: model.BaseModel{ID: 11}, DefinitionID: 7, VersionNumber: 1, Status: model.ReportVersionStatusDraft}}, nil
			}
			repository.validateReferences = func(context.Context, *gorm.DB, uint, []model.ReportGrant) error {
				referenceCalls++
				return referenceError
			}

			if err := test.run(repository); !errors.Is(err, ErrInvalidDraft) {
				t.Fatalf("operation error = %v, want ErrInvalidDraft", err)
			}
			if referenceCalls != 1 {
				t.Fatalf("reference validation calls = %d, want 1", referenceCalls)
			}
		})
	}
}

func TestDraftAuditContainsOnlySafeMetadata(t *testing.T) {
	draft := validDraft()
	draft.Parameters = []model.ReportParameter{{
		ParameterCode: "secret", Position: 1, Sensitive: true,
		DefaultValueJSON: model.JSONText(`"must-not-appear"`),
	}}
	draft.Columns = []model.ReportColumn{{LogicalCode: "orderNo", FieldID: "field-1"}}
	draft.Grants = []model.ReportGrant{{SubjectType: "ROLE", SubjectID: 2}}

	audit := newDraftAudit("REPORT_DRAFT_UPDATE", 8, 7, 4, draft)
	if audit.ActorUserID != 8 || audit.TargetID != 7 || audit.TargetType != "REPORT_DEFINITION" || audit.Action != "REPORT_DRAFT_UPDATE" {
		t.Fatalf("audit identity = %#v", audit)
	}
	if len(audit.RequestID) != 36 {
		t.Fatalf("audit request id = %q", audit.RequestID)
	}
	if strings.Contains(string(audit.DetailJSON), "must-not-appear") || strings.Contains(string(audit.DetailJSON), "default") {
		t.Fatalf("audit leaked configuration: %s", audit.DetailJSON)
	}
	var detail reportDraftAuditDetail
	if err := json.Unmarshal([]byte(audit.DetailJSON), &detail); err != nil {
		t.Fatalf("decode audit detail: %v", err)
	}
	if detail.VersionNumber != 4 || detail.ParameterCount != 1 || detail.ColumnCount != 1 || detail.GrantCount != 1 {
		t.Fatalf("audit detail = %#v", detail)
	}
}

func TestFinalizeReportMutationPropagatesAuditFailure(t *testing.T) {
	auditError := errors.New("audit unavailable")
	auditCalls := 0
	err := finalizeReportMutation(context.Background(), newDryRunDB(t), model.ReportAudit{}, func(context.Context, *gorm.DB, model.ReportAudit) error {
		auditCalls++
		return auditError
	})
	if !errors.Is(err, auditError) {
		t.Fatalf("finalizeReportMutation() error = %v, want audit error", err)
	}
	if auditCalls != 1 {
		t.Fatalf("audit calls = %d, want 1", auditCalls)
	}
}

func TestListDraftsUsesBoundedKeysetQuery(t *testing.T) {
	db := newDryRunDB(t)
	statement := buildDraftListQuery(db.Session(&gorm.Session{DryRun: true}), 5,
		DraftListQuery{AfterID: 10, Limit: 20, Search: `a%b_`}).Scan(&[]draftSummaryRecord{})
	sqlText := statement.Statement.SQL.String()
	for _, fragment := range []string{
		"versions.version_number AS lock_version", "owner_user_id = ?", "status IN", "id > ?",
		"ORDER BY definitions.id ASC", "LIMIT 21",
	} {
		if !strings.Contains(sqlText, fragment) {
			t.Fatalf("list SQL %q does not contain %q", sqlText, fragment)
		}
	}
	if got := escapeLike(`a%b_`); got != `a\%b\_` {
		t.Fatalf("escapeLike() = %q", got)
	}
}

func TestNewVersionRecordPreservesResultIdentityContract(t *testing.T) {
	record := newVersionRecord(model.ReportVersion{
		BaseModel:         model.BaseModel{ID: 99},
		DefinitionID:      7,
		VersionNumber:     4,
		ResultRunIDColumn: "EXECUTION_ID",
		ResultRowIDColumn: "RESULT_NO",
	})
	if record.ID != 0 || record.DefinitionID != 7 || record.VersionNumber != 4 ||
		record.ResultRunIDColumn != "EXECUTION_ID" || record.ResultRowIDColumn != "RESULT_NO" {
		t.Fatalf("newVersionRecord() = %#v", record.ReportVersion)
	}
}

func TestNormalizeNewDraftStartsAtVersionOne(t *testing.T) {
	draft := validDraft()
	draft.Version.VersionNumber = 99
	normalizeNewDraft(draft)
	if draft.Version.VersionNumber != 1 {
		t.Fatalf("VersionNumber = %d, want 1", draft.Version.VersionNumber)
	}
}

func TestRecordTableNamesUseExistingReportTables(t *testing.T) {
	tests := []struct{ got, want string }{
		{(definitionRecord{}).TableName(), "report_definitions"},
		{(versionRecord{}).TableName(), "report_versions"},
		{(parameterRecord{}).TableName(), "report_parameters"},
		{(columnRecord{}).TableName(), "report_columns"},
		{(grantRecord{}).TableName(), "report_grants"},
	}
	for _, test := range tests {
		if test.got != test.want {
			t.Fatalf("TableName() = %q, want %q", test.got, test.want)
		}
	}
}

func validDraft() *Draft {
	return &Draft{
		Definition: model.ReportDefinition{
			Code: "orders", Name: "订单报表", DatasourceID: 3, OwnerUserID: 8,
			Status: model.ReportDefinitionStatusDraft, CreatedBy: 8, UpdatedBy: 8,
		},
		Version: model.ReportVersion{Status: model.ReportVersionStatusDraft, VersionNumber: 1, CreatedBy: 8},
	}
}

func newDryRunDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(mysql.New(mysql.Config{
		Conn: &sql.DB{}, SkipInitializeWithVersion: true,
	}), &gorm.Config{DryRun: true, DisableAutomaticPing: true, SkipDefaultTransaction: true})
	if err != nil {
		t.Fatalf("open dry-run database: %v", err)
	}
	return db
}
