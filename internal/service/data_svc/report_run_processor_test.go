package data_svc

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"gin-biz-web-api/internal/reporting"
	"gin-biz-web-api/internal/reportrepo"
	"gin-biz-web-api/model"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestReportRunProcessorExecutesProcedureOnceAndPersistsSuccess(t *testing.T) {
	store := newFakeReportExecutionStore()
	executor := &fakeReportProcedureExecutor{rowCount: 18}
	processor := newTestReportRunProcessor(store, executor)
	if err := processor.Process(t.Context(), 31, 3, true); err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if executor.calls != 1 || executor.values["runId"] != "1234" || store.succeeded != 1 || store.unknown != 0 || store.failed != 0 {
		t.Fatalf("calls=%d succeeded=%d unknown=%d failed=%d", executor.calls, store.succeeded, store.unknown, store.failed)
	}
}

func TestReportRunProcessorWaitsWithoutConsumingFailureRetry(t *testing.T) {
	store := newFakeReportExecutionStore()
	store.beginDisposition = reportrepo.RunDispositionBusy
	processor := newTestReportRunProcessor(store, &fakeReportProcedureExecutor{})
	err := processor.Process(t.Context(), 31, 0, true)
	if !errors.Is(err, ErrReportRunWaitingForSnapshot) || store.failed != 0 {
		t.Fatalf("Process() error = %v, failed = %d", err, store.failed)
	}
	var hint interface{ RetryDelay() time.Duration }
	if !errors.As(err, &hint) || hint.RetryDelay() != 15*time.Second {
		t.Fatalf("waiting error retry hint = %v", err)
	}
}

func TestReportRunProcessorRequestsTargetedCleanupForStaleResultPurge(t *testing.T) {
	now := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	expired := now.Add(-time.Minute)
	store := newFakeReportExecutionStore()
	store.beginDisposition = reportrepo.RunDispositionBusy
	store.blocker = &reportrepo.RunExecutionBlocker{
		RunID: 24, Status: model.ReportRunStatusResultPurging, ResultExpiresAt: &expired,
	}
	processor := newTestReportRunProcessor(store, &fakeReportProcedureExecutor{})
	err := processor.Process(t.Context(), 31, 0, true)
	cleanupRunID, ok := ReportRunCleanupTarget(err)
	if !errors.Is(err, ErrReportRunWaitingForSnapshot) || !ok || cleanupRunID != 24 {
		t.Fatalf("error=%v cleanupRunID=%d ok=%v", err, cleanupRunID, ok)
	}

	activeLease := now.Add(time.Minute)
	store.blocker.LeaseExpiresAt = &activeLease
	err = processor.Process(t.Context(), 31, 0, true)
	if cleanupRunID, ok = ReportRunCleanupTarget(err); ok || cleanupRunID != 0 {
		t.Fatalf("active cleanup lease target=%d ok=%v", cleanupRunID, ok)
	}
}

func TestReportRunProcessorFailsQueuedRunWhenSnapshotWaitExpires(t *testing.T) {
	store := newFakeReportExecutionStore()
	store.beginDisposition = reportrepo.RunDispositionBusy
	processor := newTestReportRunProcessor(store, &fakeReportProcedureExecutor{})
	err := processor.Process(t.Context(), 31, 3, false)
	if !errors.Is(err, ErrReportRunProcessNonRetryable) || store.queuedFailed != 1 {
		t.Fatalf("Process() error = %v, queued failed = %d", err, store.queuedFailed)
	}
}

func TestReportRunProcessorUsesExecutionAttemptsForFailureRetryBudget(t *testing.T) {
	tests := []struct {
		name         string
		attempt      int
		wantReleased int
		wantFailed   int
	}{
		{name: "third retry remains available", attempt: 3, wantReleased: 1},
		{name: "fourth attempt exhausts retries", attempt: 4, wantFailed: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newFakeReportExecutionStore()
			store.runAttempt = tt.attempt
			store.loadErr = errors.New("temporary contract load failure")
			processor := newTestReportRunProcessor(store, &fakeReportProcedureExecutor{})
			err := processor.Process(t.Context(), 31, 3, true)
			if store.released != tt.wantReleased || store.failed != tt.wantFailed {
				t.Fatalf("Process() error = %v, released = %d, failed = %d", err, store.released, store.failed)
			}
		})
	}
}

func TestReportRunProcessorBuildsSingleJSONCursorPayload(t *testing.T) {
	store := newFakeReportExecutionStore()
	store.runtime.Definition.ID = 1234
	store.runtime.Version.ExecutionMode = model.ReportExecutionModeRefCursor
	store.runtime.Parameters = nil
	store.runtime.Run.NormalizedParametersJSON = model.JSONText(`{"c_store_id":[],"c_supplier_id":["a","b"],"datein_begin":"20260504","datein_end":""}`)
	executor := &fakeReportProcedureExecutor{rowCount: 2}
	processor := newTestReportRunProcessor(store, executor)

	if err := processor.Process(t.Context(), 31, 3, true); err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if got, want := executor.jsonPayload, `{"report_id":31,"conditions":{"c_store_id":[],"c_supplier_id":["a","b"],"datein_begin":"20260504","datein_end":""}}`; got != want {
		t.Fatalf("JSON payload = %s, want %s", got, want)
	}
}

func TestReportRunProcessorBuildsJSONResultTablePayloadWithRunID(t *testing.T) {
	store := newFakeReportExecutionStore()
	store.runtime.Definition.ID = 1234
	store.runtime.Version.ExecutionMode = model.ReportExecutionModeTableSnapshot
	store.runtime.Version.JSONInputArgName = "P_PAYLOAD"
	store.runtime.Parameters = nil
	store.runtime.Run.NormalizedParametersJSON = model.JSONText(`{"c_supplier_id":["a","b"],"datein_begin":"20260504"}`)
	executor := &fakeReportProcedureExecutor{rowCount: 2}
	processor := newTestReportRunProcessor(store, executor)

	if err := processor.Process(t.Context(), 31, 3, true); err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if got, want := executor.jsonPayload, `{"report_id":31,"conditions":{"c_supplier_id":["a","b"],"datein_begin":"20260504"}}`; got != want {
		t.Fatalf("JSON payload = %s, want %s", got, want)
	}
}

func TestLogReportOracleProcedureInputIncludesActualJSON(t *testing.T) {
	core, observed := observer.New(zap.InfoLevel)
	previous := zap.L()
	zap.ReplaceGlobals(zap.New(core))
	defer zap.ReplaceGlobals(previous)

	request := reportProcedureExecutionRequest{
		Runtime: reportrepo.RuntimeContract{
			Run: model.ReportRun{BaseModel: model.BaseModel{ID: 31}, RunUUID: "11111111-1111-4111-8111-111111111111", DefinitionID: 13},
			Version: model.ReportVersion{
				ProcedureOwner: "YL_TEST", PackageName: "PKG_REPORT", ProcedureName: "BUILD_REPORT",
				ExecutionMode: model.ReportExecutionModeTableSnapshot, JSONInputArgName: "P_JSON",
			},
		},
		JSONPayload: `{"report_id":31,"conditions":{"store_id":["a1","b2"]}}`,
	}

	logReportOracleProcedureInput(request)
	entries := observed.FilterMessage("调用Oracle报表存储过程").All()
	if len(entries) != 1 {
		t.Fatalf("log entries = %d", len(entries))
	}
	fields := entries[0].ContextMap()
	if fields["oracle_procedure"] != "YL_TEST.PKG_REPORT.BUILD_REPORT" || fields["input_argument"] != "P_JSON" || fields["actual_input_json"] != request.JSONPayload {
		t.Fatalf("log fields = %#v", fields)
	}
}

func TestReportProcedureLogValuesRedactsSensitiveLegacyParameters(t *testing.T) {
	values := map[string]interface{}{"storeCode": "S001", "secret": "plain-text"}
	logged := reportProcedureLogValues([]reporting.ParameterDefinition{{Code: "secret", Sensitive: true}}, values)
	if logged["storeCode"] != "S001" || logged["secret"] != "[REDACTED]" || values["secret"] != "plain-text" {
		t.Fatalf("logged=%#v values=%#v", logged, values)
	}
}

func TestLogReportOracleExecutionFailureDoesNotClaimInputWasReceived(t *testing.T) {
	core, observed := observer.New(zap.ErrorLevel)
	previous := zap.L()
	zap.ReplaceGlobals(zap.New(core))
	defer zap.ReplaceGlobals(previous)

	oracleErr := errors.New("ORA-06502: PL/SQL numeric or value error")
	request := reportProcedureExecutionRequest{
		Runtime: reportrepo.RuntimeContract{
			Run:     model.ReportRun{BaseModel: model.BaseModel{ID: 31}},
			Version: model.ReportVersion{ProcedureOwner: "YL_TEST", ProcedureName: "BUILD_REPORT", JSONInputArgName: "P_JSON"},
		},
		JSONPayload: `{"report_id":31,"conditions":{}}`,
	}
	logReportOracleExecutionFailure(request, oracleErr)
	entries := observed.FilterMessage("Oracle报表执行失败").All()
	if len(entries) != 1 {
		t.Fatalf("log entries = %d", len(entries))
	}
	fields := entries[0].ContextMap()
	if fields["error"] != oracleErr.Error() {
		t.Fatalf("log fields = %#v", fields)
	}
	if _, exists := fields["actual_input_json"]; exists {
		t.Fatalf("failed execution log must not claim that Oracle received input: %#v", fields)
	}
}

func TestReportRunProcessorMarksUnknownWithoutRetryingAfterCommitAmbiguity(t *testing.T) {
	store := newFakeReportExecutionStore()
	executor := &fakeReportProcedureExecutor{err: errOracleCommitOutcomeUnknown}
	processor := newTestReportRunProcessor(store, executor)
	err := processor.Process(t.Context(), 31, 3, true)
	if !errors.Is(err, ErrReportRunProcessNonRetryable) {
		t.Fatalf("Process() error = %v, want non-retryable", err)
	}
	if executor.calls != 1 || store.unknown != 1 || store.failed != 0 || store.unknownCode != "ORACLE_COMMIT_OUTCOME_UNKNOWN" {
		t.Fatalf("calls=%d unknown=%d failed=%d code=%q", executor.calls, store.unknown, store.failed, store.unknownCode)
	}
}

func TestReportRunProcessorCommitAmbiguityOverridesConcurrentCancellation(t *testing.T) {
	store := newFakeReportExecutionStore()
	store.cancelOnInspectAfterOracle = true
	executor := &fakeReportProcedureExecutor{err: errOracleCommitOutcomeUnknown}
	processor := newTestReportRunProcessor(store, executor)
	err := processor.Process(t.Context(), 31, 3, true)
	if !errors.Is(err, ErrReportRunProcessNonRetryable) || store.unknown != 1 || store.cancelled != 0 {
		t.Fatalf("error=%v unknown=%d cancelled=%d", err, store.unknown, store.cancelled)
	}
}

func TestReportRunProcessorRoutesUnknownRunsToReconciliation(t *testing.T) {
	store := newFakeReportExecutionStore()
	store.beginDisposition = reportrepo.RunDispositionReconcile
	processor := newTestReportRunProcessor(store, &fakeReportProcedureExecutor{})
	called := 0
	processor.reconcileOne = func(context.Context, uint) error { called++; return nil }
	if err := processor.Process(t.Context(), 31, 3, true); err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if called != 1 {
		t.Fatalf("reconcile calls = %d", called)
	}
}

func TestReportRunProcessorHeartbeatCancelsOracleExecution(t *testing.T) {
	store := newFakeReportExecutionStore()
	store.cancelOnHeartbeat = true
	executor := &fakeReportProcedureExecutor{waitForCancel: true}
	processor := newTestReportRunProcessor(store, executor)
	processor.heartbeatInterval = time.Millisecond
	processor.stateTimeout = time.Second
	if err := processor.Process(t.Context(), 31, 3, true); err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if executor.calls != 1 || store.cancelled != 1 {
		t.Fatalf("calls=%d cancelled=%d", executor.calls, store.cancelled)
	}
}

func TestReportRunProcessorRejectsRuntimeContractBeforeOracleStart(t *testing.T) {
	store := newFakeReportExecutionStore()
	store.loadErr = errors.New("hash mismatch")
	executor := &fakeReportProcedureExecutor{}
	processor := newTestReportRunProcessor(store, executor)
	err := processor.Process(t.Context(), 31, 0, true)
	if !errors.Is(err, ErrReportRunProcessNonRetryable) || executor.calls != 0 || store.failed != 1 || store.oracleStarted != 0 {
		t.Fatalf("error=%v calls=%d failed=%d oracleStarted=%d", err, executor.calls, store.failed, store.oracleStarted)
	}
}

func TestReportRunProcessorPreservesOracleCauseForWorkerAndPersistsSafeFailure(t *testing.T) {
	store := newFakeReportExecutionStore()
	oracleErr := fmt.Errorf("execute oracle report procedure: %w", fakeReportOracleError{code: 20201, message: "报表查询出错，请联系it人员！"})
	executor := &fakeReportProcedureExecutor{err: oracleErr}
	processor := newTestReportRunProcessor(store, executor)

	err := processor.Process(t.Context(), 31, 3, true)
	if !errors.Is(err, ErrReportRunProcessNonRetryable) {
		t.Fatalf("Process() error = %v, want non-retryable", err)
	}
	if !errors.Is(err, oracleErr) {
		t.Fatalf("Process() error = %v, want Oracle cause", err)
	}
	if !strings.Contains(err.Error(), oracleErr.Error()) {
		t.Fatalf("Process() error = %q, want Worker-visible Oracle details", err)
	}
	if store.failed != 1 || store.failedCode != "ORACLE_EXECUTION_FAILED" || store.failedMessage != "ORA-20201：报表查询出错，请联系it人员！" {
		t.Fatalf("failed=%d code=%q message=%q", store.failed, store.failedCode, store.failedMessage)
	}
}

func TestSafeReportOracleExecutionMessage(t *testing.T) {
	longMessage := strings.Repeat("界", 451)
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "wrapped user defined Oracle error", err: fmt.Errorf("execute report: %w", fakeReportOracleError{code: 20201, message: "报表查询出错，请联系it人员！"}), want: "ORA-20201：报表查询出错，请联系it人员！"},
		{name: "minimum application error", err: fakeReportOracleError{code: 20000, message: "最小错误码"}, want: "ORA-20000：最小错误码"},
		{name: "maximum application error", err: fakeReportOracleError{code: 20999, message: "最大错误码"}, want: "ORA-20999：最大错误码"},
		{name: "code below application range", err: fakeReportOracleError{code: 19999, message: "private detail"}, want: "Oracle存储过程执行失败"},
		{name: "code above application range", err: fakeReportOracleError{code: 21000, message: "private detail"}, want: "Oracle存储过程执行失败"},
		{name: "plain text cannot spoof Oracle type", err: errors.New("ORA-20201: private detail"), want: "Oracle存储过程执行失败"},
		{name: "message uses first line and removes controls", err: fakeReportOracleError{code: 20201, message: " 对外\t提示\n内部对象名 "}, want: "ORA-20201：对外提示"},
		{name: "empty public message", err: fakeReportOracleError{code: 20201, message: "\t\nprivate detail"}, want: "Oracle存储过程执行失败"},
		{name: "long message is rune truncated", err: fakeReportOracleError{code: 20201, message: longMessage}, want: "ORA-20201：" + strings.Repeat("界", 450)},
		{name: "nil error", err: nil, want: "Oracle存储过程执行失败"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := safeReportOracleExecutionMessage(test.err); got != test.want {
				t.Fatalf("safeReportOracleExecutionMessage() = %q, want %q", got, test.want)
			}
		})
	}
}

type fakeReportOracleError struct {
	code    int
	message string
}

func (err fakeReportOracleError) Error() string {
	return fmt.Sprintf("ORA-%05d: %s", err.code, err.message)
}
func (err fakeReportOracleError) Code() int       { return err.code }
func (err fakeReportOracleError) Message() string { return err.message }

func newTestReportRunProcessor(store *fakeReportExecutionStore, executor *fakeReportProcedureExecutor) *ReportRunProcessor {
	processor := NewReportRunProcessorWithDependencies(store, fakeReportCredentialDecryptor{}, fakeRunParameterDecryptor{}, executor)
	processor.heartbeatInterval = time.Hour
	processor.stateTimeout = time.Second
	processor.now = func() time.Time { return time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC) }
	return processor
}

type fakeReportExecutionStore struct {
	mu                         sync.Mutex
	runtime                    *reportrepo.RuntimeContract
	loadErr                    error
	heartbeatControl           reportrepo.RunControl
	cancelOnHeartbeat          bool
	cancelOnInspectAfterOracle bool
	oracleStarted              int
	succeeded                  int
	failed                     int
	failedCode                 string
	failedMessage              string
	cancelled                  int
	unknown                    int
	unknownCode                string
	beginDisposition           reportrepo.RunDisposition
	blocker                    *reportrepo.RunExecutionBlocker
	runAttempt                 int
	released                   int
	queuedFailed               int
}

func newFakeReportExecutionStore() *fakeReportExecutionStore {
	return &fakeReportExecutionStore{
		heartbeatControl: reportrepo.RunControlContinue,
		runtime: &reportrepo.RuntimeContract{
			Run: model.ReportRun{BaseModel: model.BaseModel{ID: 31}, RunUUID: "11111111-1111-4111-8111-111111111111", DefinitionID: 1234, NormalizedParametersJSON: model.JSONText(`{"storeCode":"S001"}`)},
			Parameters: []model.ReportParameter{
				{ParameterCode: "runId", ProcedureArgName: "P_RUN_ID", Position: 1, Direction: "IN", LogicalType: "string", OracleType: "VARCHAR2", Cardinality: "SINGLE", Required: true, SystemInjected: true, NullPolicy: "TYPED_NULL"},
				{ParameterCode: "storeCode", ProcedureArgName: "P_STORE_CODE", Position: 2, Direction: "IN", LogicalType: "string", OracleType: "VARCHAR2", Cardinality: "SINGLE", Required: true, NullPolicy: "TYPED_NULL"},
			},
		},
	}
}

func (store *fakeReportExecutionStore) BeginExecution(context.Context, uint, string, string, time.Time, time.Duration) (*reportrepo.RunLease, error) {
	disposition := store.beginDisposition
	if disposition == reportrepo.RunDispositionUnknown {
		disposition = reportrepo.RunDispositionAcquired
	}
	attempt := store.runAttempt
	if attempt == 0 {
		attempt = 1
	}
	return &reportrepo.RunLease{Disposition: disposition, Run: model.ReportRun{Attempt: attempt}, Blocker: store.blocker}, nil
}
func (store *fakeReportExecutionStore) HeartbeatExecution(context.Context, uint, string, time.Time, time.Duration) (reportrepo.RunControl, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.cancelOnHeartbeat {
		store.heartbeatControl = reportrepo.RunControlCancelRequested
	}
	return store.heartbeatControl, nil
}
func (store *fakeReportExecutionStore) InspectExecution(context.Context, uint, string) (reportrepo.RunControl, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.cancelOnInspectAfterOracle && store.oracleStarted > 0 {
		return reportrepo.RunControlCancelRequested, nil
	}
	return store.heartbeatControl, nil
}
func (store *fakeReportExecutionStore) MarkOracleExecutionStarted(context.Context, uint, string, time.Time) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.oracleStarted++
	return nil
}
func (store *fakeReportExecutionStore) MarkExecutionSucceeded(context.Context, uint, string, int64, time.Time, time.Time) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.succeeded++
	return nil
}
func (store *fakeReportExecutionStore) ConfirmExecutionSucceeded(context.Context, uint, int64) (bool, error) {
	return false, nil
}
func (store *fakeReportExecutionStore) MarkExecutionFailed(_ context.Context, _ uint, _, code, message string, _ time.Time) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.failed++
	store.failedCode = code
	store.failedMessage = message
	return nil
}
func (store *fakeReportExecutionStore) MarkExecutionCancelled(context.Context, uint, string, time.Time) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.cancelled++
	return nil
}
func (store *fakeReportExecutionStore) MarkExecutionUnknown(_ context.Context, _ uint, _, code, _ string, _ time.Time) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.unknown++
	store.unknownCode = code
	return nil
}
func (store *fakeReportExecutionStore) ReleaseExecutionForRetry(context.Context, uint, string, time.Time) error {
	store.released++
	return nil
}
func (store *fakeReportExecutionStore) MarkQueuedExecutionFailed(context.Context, uint, string, string, time.Time) error {
	store.queuedFailed++
	return nil
}
func (store *fakeReportExecutionStore) ListReconciliationCandidates(context.Context, time.Time, int) ([]uint, error) {
	return nil, nil
}
func (store *fakeReportExecutionStore) BeginReconciliation(context.Context, uint, string, string, time.Time, time.Duration) (*reportrepo.RunLease, error) {
	return &reportrepo.RunLease{Disposition: reportrepo.RunDispositionTerminal}, nil
}
func (store *fakeReportExecutionStore) MarkReconciliationSucceeded(context.Context, uint, string, int64, time.Time, time.Time) error {
	return nil
}
func (store *fakeReportExecutionStore) MarkReconciliationPending(context.Context, uint, string, string, string, time.Time) error {
	return nil
}
func (store *fakeReportExecutionStore) RecoverExpiredPreOracleRuns(context.Context, time.Time, int) (int64, error) {
	return 0, nil
}
func (store *fakeReportExecutionStore) ListExpiredQueuedRuns(context.Context, time.Time, int) ([]uint, error) {
	return nil, nil
}
func (store *fakeReportExecutionStore) ListQueuedRunsMissingDelivery(context.Context, int) ([]uint, error) {
	return nil, nil
}
func (store *fakeReportExecutionStore) EnsureRunQueued(context.Context, uint, time.Time) error {
	return nil
}
func (store *fakeReportExecutionStore) LoadRuntimeContract(context.Context, uint, string) (*reportrepo.RuntimeContract, error) {
	return store.runtime, store.loadErr
}

type fakeReportCredentialDecryptor struct{}

func (fakeReportCredentialDecryptor) Decrypt(string, string) (string, error) { return "password", nil }

type fakeRunParameterDecryptor struct{}

func (fakeRunParameterDecryptor) Decrypt(string, string) ([]byte, error) { return []byte(`{}`), nil }

type fakeReportProcedureExecutor struct {
	mu            sync.Mutex
	calls         int
	rowCount      int64
	err           error
	waitForCancel bool
	jsonPayload   string
	values        map[string]interface{}
}

func (executor *fakeReportProcedureExecutor) Execute(ctx context.Context, request reportProcedureExecutionRequest, password string) (int64, error) {
	executor.mu.Lock()
	executor.calls++
	executor.jsonPayload = request.JSONPayload
	executor.values = request.Values
	executor.mu.Unlock()
	if password != "password" {
		return 0, errors.New("unexpected execution request")
	}
	if isJSONInputReport(request.Runtime.Version) {
		if request.JSONPayload == "" || len(request.Values) != 0 {
			return 0, errors.New("unexpected JSON execution request")
		}
	} else if request.Values["runId"] == nil || request.Values["storeCode"] != "S001" {
		return 0, errors.New("unexpected execution request")
	}
	if executor.waitForCancel {
		<-ctx.Done()
		return 0, ctx.Err()
	}
	return executor.rowCount, executor.err
}
