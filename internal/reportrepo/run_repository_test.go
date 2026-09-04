package reportrepo

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"gin-biz-web-api/model"

	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"
)

func TestActorCanRunReportSupportsOwnerAndUserGrant(t *testing.T) {
	grants := []model.ReportGrant{{SubjectType: "USER", SubjectID: 7, ActionsJSON: model.JSONText(`["QUERY"]`)}}
	authority, allowed, err := actorCanRunReport(t.Context(), newDryRunDB(t), 7, 9, ReportActionQuery, grants)
	if err != nil || !allowed || authority.Source != "USER" || len(authority.Grants) != 1 {
		t.Fatalf("direct user grant = %#v %t, %v", authority, allowed, err)
	}
	authority, allowed, err = actorCanRunReport(t.Context(), newDryRunDB(t), 9, 9, ReportActionQuery, nil)
	if err != nil || !allowed || authority.Source != "OWNER" || len(authority.Grants) != 0 {
		t.Fatalf("owner grant = %#v %t, %v", authority, allowed, err)
	}
	authority, allowed, err = actorCanRunReport(t.Context(), newDryRunDB(t), 8, 9, ReportActionQuery, grants)
	if err != nil || allowed {
		t.Fatalf("ungranted actor = %#v %t, %v", authority, allowed, err)
	}
}

func TestConfiguredCategoryAccessDoesNotAllowOwnerBypass(t *testing.T) {
	published := &PublishedReport{
		Definition:               model.ReportDefinition{OwnerUserID: 9},
		CategoryAccessConfigured: true,
	}
	authority, allowed, err := actorCanRunReport(t.Context(), newDryRunDB(t), 9, published.authorizationOwner(), ReportActionQuery, nil)
	if err != nil || allowed || authority.Source != "" {
		t.Fatalf("configured empty category owner access = %#v %t, %v", authority, allowed, err)
	}

	published.CategoryAccessConfigured = false
	authority, allowed, err = actorCanRunReport(t.Context(), newDryRunDB(t), 9, published.authorizationOwner(), ReportActionQuery, nil)
	if err != nil || !allowed || authority.Source != "OWNER" {
		t.Fatalf("legacy owner access = %#v %t, %v", authority, allowed, err)
	}
}

func TestMatchRoleGrantsUsesOnlyActiveMemberships(t *testing.T) {
	grants := []model.ReportGrant{
		{SubjectType: "ROLE", SubjectID: 2, ActionsJSON: model.JSONText(`["QUERY"]`)},
		{SubjectType: "ROLE", SubjectID: 3, ActionsJSON: model.JSONText(`["QUERY"]`)},
		{SubjectType: "ROLE", SubjectID: 4, ActionsJSON: model.JSONText(`["EXPORT"]`)},
	}
	matched := matchRoleGrants(grants, []uint{2, 4}, ReportActionQuery)
	if len(matched) != 1 || matched[0].SubjectID != 2 {
		t.Fatalf("matched grants = %#v", matched)
	}
}

func TestGrantAllowsActionRejectsMalformedOrDifferentAction(t *testing.T) {
	for _, grant := range []model.ReportGrant{
		{ActionsJSON: model.JSONText(`not-json`)},
		{ActionsJSON: model.JSONText(`["EXPORT"]`)},
	} {
		if grantAllowsAction(grant, ReportActionQuery) {
			t.Fatalf("grant unexpectedly allows query: %#v", grant)
		}
	}
}

func TestCreateRunRejectsUnsafeOutboxBeforeTransaction(t *testing.T) {
	db, transactionState := newTransactionDB(t)
	repository := New(db)
	repository.transact = runTransaction
	command := validRunCommand()
	command.Outbox.PayloadJSON = model.JSONText(`{"run_id":0,"secret":"leak"}`)
	if err := repository.CreateRun(t.Context(), 17, 9, command); !errors.Is(err, ErrInvalidRun) {
		t.Fatalf("CreateRun() error = %v, want ErrInvalidRun", err)
	}
	if transactionState.begins != 0 {
		t.Fatalf("transaction began for invalid command: %#v", transactionState)
	}
}

func TestCreateRunCommitsRunOutboxAndAuditAtomically(t *testing.T) {
	repository, transactionState := runTestRepository(t)
	command := validRunCommand()
	repository.createReportRun = func(_ context.Context, _ *gorm.DB, run *model.ReportRun) error {
		run.ID = 31
		run.CreatedAt = time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
		return nil
	}
	repository.createRunOutbox = func(_ context.Context, _ *gorm.DB, outbox *model.AsyncJobOutbox) error {
		if string(outbox.PayloadJSON) != `{"run_id":31}` {
			t.Fatalf("outbox payload = %s", outbox.PayloadJSON)
		}
		outbox.ID = 44
		return nil
	}
	repository.writeAudit = func(_ context.Context, _ *gorm.DB, audit model.ReportAudit) error {
		if audit.Action != "REPORT_RUN_CREATE" || audit.TargetID != 31 || strings.Contains(strings.ToLower(string(audit.DetailJSON)), "secret") {
			t.Fatalf("audit = %#v", audit)
		}
		return nil
	}
	if err := repository.CreateRun(t.Context(), 17, 9, command); err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	if transactionState.begins != 1 || transactionState.commits != 1 || transactionState.rollbacks != 0 {
		t.Fatalf("transaction state = %#v", transactionState)
	}
	if command.Run.ID != 31 || command.Outbox.ID != 44 || !json.Valid([]byte(command.Run.PermissionSnapshotJSON)) ||
		!json.Valid([]byte(command.Run.PresentationSnapshotJSON)) {
		t.Fatalf("committed command = %#v", command)
	}
}

func TestCreateRunRollsBackAtEveryWriteBoundary(t *testing.T) {
	steps := []string{"create run", "create outbox", "write audit"}
	for _, failedStep := range steps {
		t.Run(failedStep, func(t *testing.T) {
			repository, transactionState := runTestRepository(t)
			injected := errors.New("injected failure")
			command := validRunCommand()
			original := *command
			repository.createReportRun = func(_ context.Context, _ *gorm.DB, run *model.ReportRun) error {
				if failedStep == steps[0] {
					return injected
				}
				run.ID = 31
				return nil
			}
			repository.createRunOutbox = func(context.Context, *gorm.DB, *model.AsyncJobOutbox) error {
				if failedStep == steps[1] {
					return injected
				}
				return nil
			}
			repository.writeAudit = func(context.Context, *gorm.DB, model.ReportAudit) error {
				if failedStep == steps[2] {
					return injected
				}
				return nil
			}
			if err := repository.CreateRun(t.Context(), 17, 9, command); !errors.Is(err, injected) {
				t.Fatalf("CreateRun() error = %v, want injected failure", err)
			}
			if transactionState.begins != 1 || transactionState.rollbacks != 1 || transactionState.commits != 0 {
				t.Fatalf("transaction state = %#v", transactionState)
			}
			if command.Run.ID != original.Run.ID || command.Run.PermissionSnapshotJSON != original.Run.PermissionSnapshotJSON ||
				command.Outbox.PayloadJSON != original.Outbox.PayloadJSON {
				t.Fatalf("failed transaction mutated command: %#v", command)
			}
		})
	}
}

func TestCreateRunRejectsBusyReportSnapshotSlot(t *testing.T) {
	repository, transactionState := runTestRepository(t)
	repository.prepareRunSlot = func(context.Context, *gorm.DB, uint, time.Time) ([]uint, error) {
		return nil, ErrReportRunBusy
	}
	if err := repository.CreateRun(t.Context(), 17, 9, validRunCommand()); !errors.Is(err, ErrReportRunBusy) {
		t.Fatalf("CreateRun() error = %v, want ErrReportRunBusy", err)
	}
	if transactionState.begins != 1 || transactionState.rollbacks != 1 || transactionState.commits != 0 {
		t.Fatalf("transaction state = %#v", transactionState)
	}
}

func TestRunSnapshotsContainNoCredentialFields(t *testing.T) {
	permission, err := encodeRunPermissionSnapshot(17, ReportActionQuery, runAuthority{Source: "ROLE", Grants: []model.ReportGrant{{SubjectType: "ROLE", SubjectID: 2, ActionsJSON: model.JSONText(`["QUERY"]`)}}})
	if err != nil || !json.Valid([]byte(permission)) {
		t.Fatalf("permission snapshot = %q, %v", permission, err)
	}
	presentation, err := encodeRunPresentationSnapshot([]model.ReportColumn{{LogicalCode: "orderNo", DatabaseColumn: "ORDER_NO", ExcelHeader: "订单号"}})
	if err != nil || !json.Valid([]byte(presentation)) {
		t.Fatalf("presentation snapshot = %q, %v", presentation, err)
	}
	combined := strings.ToLower(string(permission) + string(presentation))
	for _, forbidden := range []string{"password", "ciphertext", "credential"} {
		if strings.Contains(combined, forbidden) {
			t.Fatalf("snapshot leaked %q: %s", forbidden, combined)
		}
	}
	for _, databaseField := range []string{"versionId", "createdAt", "updatedAt", "deletedAt"} {
		if strings.Contains(string(presentation), databaseField) {
			t.Fatalf("presentation snapshot leaked persistence field %q: %s", databaseField, presentation)
		}
	}
}

func TestFrozenRunPermissionKeepsQueryAndExportCapabilities(t *testing.T) {
	query := runAuthority{Source: "ROLE", Grants: []model.ReportGrant{{SubjectType: "ROLE", SubjectID: 2, ActionsJSON: model.JSONText(`["QUERY"]`)}}}
	export := runAuthority{Source: "ROLE", Grants: []model.ReportGrant{{SubjectType: "ROLE", SubjectID: 3, ActionsJSON: model.JSONText(`["EXPORT"]`)}}}
	snapshot, err := encodeRunPermissionCapabilities(17, query, export, true)
	if err != nil {
		t.Fatalf("encodeRunPermissionCapabilities() error = %v", err)
	}
	if !frozenRunAllowsAction(snapshot, 17, ReportActionQuery) || !frozenRunAllowsAction(snapshot, 17, ReportActionExport) {
		t.Fatalf("snapshot does not preserve both capabilities: %s", snapshot)
	}
	if frozenRunAllowsAction(snapshot, 18, ReportActionExport) {
		t.Fatal("snapshot allowed a different actor")
	}
	forged := model.JSONText(`{"actor":17,"action":"QUERY","grantedBy":"OWNER","grants":[{"subjectType":"USER","subjectId":99,"actions":["EXPORT"]}]}`)
	if frozenRunAllowsAction(forged, 17, ReportActionExport) {
		t.Fatal("legacy or forged redundant fields granted export")
	}
	unknown := model.JSONText(`{"actor":17,"action":"QUERY","actions":["QUERY","ADMIN"],"grantedBy":"USER","grants":[]}`)
	if frozenRunAllowsAction(unknown, 17, ReportActionQuery) {
		t.Fatal("snapshot with an unknown capability was accepted")
	}
	queryOnly, err := encodeRunPermissionCapabilities(17, query, runAuthority{}, false)
	if err != nil || frozenRunAllowsAction(queryOnly, 17, ReportActionExport) {
		t.Fatalf("query-only snapshot unexpectedly allows export: %s, %v", queryOnly, err)
	}
}

func TestLegacyRunPermissionRequiresExplicitLiveAuthorizationPath(t *testing.T) {
	legacy, err := encodeRunPermissionSnapshot(17, ReportActionQuery, runAuthority{Source: "ROLE", Grants: []model.ReportGrant{{SubjectType: "ROLE", SubjectID: 2, ActionsJSON: model.JSONText(`["QUERY"]`)}}})
	if err != nil {
		t.Fatalf("encodeRunPermissionSnapshot() error = %v", err)
	}
	if frozenRunAllowsAction(legacy, 17, ReportActionExport) {
		t.Fatal("legacy snapshot directly granted export")
	}
	if !frozenLegacyRunAllowsLiveAuthorization(legacy, 17) {
		t.Fatal("valid legacy query snapshot did not enter live authorization path")
	}
	for _, invalid := range []model.JSONText{
		`{"actor":18,"action":"QUERY","grantedBy":"ROLE","grants":[]}`,
		`{"actor":17,"action":"EXPORT","grantedBy":"ROLE","grants":[]}`,
		`{"actor":17,"action":"QUERY","actions":["QUERY"],"grantedBy":"ROLE","grants":[]}`,
		`{"actor":17,"action":"QUERY","grantedBy":"","grants":[]}`,
	} {
		if frozenLegacyRunAllowsLiveAuthorization(invalid, 17) {
			t.Fatalf("invalid legacy snapshot accepted: %s", invalid)
		}
	}
}

func TestWriteReportRunSuppressesInterpolatedSQLLogging(t *testing.T) {
	spy := &traceCountingLogger{}
	db := newDryRunDB(t).Session(&gorm.Session{Logger: spy})
	run := validRunCommand().Run
	run.NormalizedParametersJSON = model.JSONText(`{"customer":"private-value"}`)
	if err := writeReportRun(t.Context(), db, &run); err != nil {
		t.Fatalf("writeReportRun() error = %v", err)
	}
	if spy.traces != 0 {
		t.Fatalf("GORM logger observed %d report run SQL traces", spy.traces)
	}
}

type traceCountingLogger struct{ traces int }

func (logger *traceCountingLogger) LogMode(gormLogger.LogLevel) gormLogger.Interface { return logger }
func (logger *traceCountingLogger) Info(context.Context, string, ...interface{})     {}
func (logger *traceCountingLogger) Warn(context.Context, string, ...interface{})     {}
func (logger *traceCountingLogger) Error(context.Context, string, ...interface{})    {}
func (logger *traceCountingLogger) Trace(context.Context, time.Time, func() (string, int64), error) {
	logger.traces++
}

func validRunCommand() *CreateRunCommand {
	runUUID := "11111111-1111-4111-8111-111111111111"
	return &CreateRunCommand{
		Run: model.ReportRun{
			RunUUID: runUUID, DefinitionID: 9, VersionID: 23, RequestedBy: 17, Status: model.ReportRunStatusQueued,
			ExecutionFingerprint: strings.Repeat("d", 64), NormalizedParametersJSON: model.JSONText(`{"store":"S001"}`),
			ContractHash: strings.Repeat("a", 64), ProcedureSignatureHash: strings.Repeat("b", 64), ResultSchemaHash: strings.Repeat("c", 64),
		},
		Outbox: NewReportRunOutbox(runUUID, time.Now().UTC()),
	}
}

func runTestRepository(t *testing.T) (*Repository, *transactionDriverState) {
	t.Helper()
	db, transactionState := newTransactionDB(t)
	repository := New(db)
	repository.loadPublished = func(context.Context, *gorm.DB, uint, uint, string, bool) (*PublishedReport, error) {
		return &PublishedReport{
			Definition: model.ReportDefinition{BaseModel: model.BaseModel{ID: 9}, OwnerUserID: 17},
			Version:    model.ReportVersion{BaseModel: model.BaseModel{ID: 23}, ContractHash: strings.Repeat("a", 64), ProcedureSignatureHash: strings.Repeat("b", 64), ResultSchemaHash: strings.Repeat("c", 64)},
			Columns:    []model.ReportColumn{{FieldID: "field-1", LogicalCode: "orderNo", DatabaseColumn: "ORDER_NO", PreviewHeader: "订单号", ExcelHeader: "订单号"}},
			Grants:     []model.ReportGrant{{SubjectType: "USER", SubjectID: 17, ActionsJSON: model.JSONText(`["QUERY"]`)}},
			authority:  runAuthority{Source: "USER", Grants: []model.ReportGrant{{SubjectType: "USER", SubjectID: 17, ActionsJSON: model.JSONText(`["QUERY"]`)}}},
		}, nil
	}
	repository.validateRunSource = func(context.Context, *gorm.DB, uint) error { return nil }
	repository.prepareRunSlot = func(context.Context, *gorm.DB, uint, time.Time) ([]uint, error) { return nil, nil }
	return repository, transactionState
}
