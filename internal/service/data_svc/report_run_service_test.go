package data_svc

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"gin-biz-web-api/internal/reportrepo"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/model"
)

func TestReportRunServiceCreatesNormalizedEncryptedQueuedRun(t *testing.T) {
	store := &fakeReportRunStore{published: publishedRunFixture()}
	cipher := &fakeReportParameterCipher{version: "parameter-v1", ciphertext: "encrypted"}
	service := NewReportRunServiceWithDependencies(store, cipher)
	createdAt := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return createdAt }
	result, err := service.Create(t.Context(), 17, 9, requestbody.ReportRunCreateRequest{
		Parameters: map[string]json.RawMessage{"storeCode": json.RawMessage(`"S001"`), "secret": json.RawMessage(`"value"`)},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if result.RunUUID == "" || result.DefinitionID != 9 || result.VersionID != 23 || result.Status != model.ReportRunStatusQueued {
		t.Fatalf("result = %#v", result)
	}
	if store.command == nil || store.command.Run.ExecutionFingerprint == "" || store.command.Run.SensitiveParametersCipher != "encrypted" ||
		store.command.Run.SensitiveParametersKeyVersion != "parameter-v1" || strings.Contains(string(store.command.Run.NormalizedParametersJSON), "secret") ||
		!strings.Contains(string(store.command.Run.NormalizedParametersJSON), "storeCode") || cipher.plaintext != `{"secret":"value"}` {
		t.Fatalf("command = %#v plaintext=%s", store.command, cipher.plaintext)
	}
	if strings.Contains(string(store.command.Outbox.PayloadJSON), "secret") || store.command.Outbox.TaskType != "report:run" {
		t.Fatalf("outbox = %#v", store.command.Outbox)
	}
}

func TestReportRunServiceRejectsSystemParameterAndInvalidAllowedValue(t *testing.T) {
	for _, parameters := range []map[string]json.RawMessage{
		{"runId": json.RawMessage(`"client-controlled"`), "storeCode": json.RawMessage(`"S001"`), "secret": json.RawMessage(`"value"`)},
		{"storeCode": json.RawMessage(`"S999"`), "secret": json.RawMessage(`"value"`)},
	} {
		store := &fakeReportRunStore{published: publishedRunFixture()}
		service := NewReportRunServiceWithDependencies(store, &fakeReportParameterCipher{})
		if _, err := service.Create(t.Context(), 17, 9, requestbody.ReportRunCreateRequest{Parameters: parameters}); !errors.Is(err, ErrReportRunInvalid) {
			t.Fatalf("Create() error = %v, want ErrReportRunInvalid", err)
		}
		if store.command != nil {
			t.Fatal("invalid parameters created a run")
		}
	}
}

func TestReportRunServiceDoesNotEncryptEmptySensitiveObject(t *testing.T) {
	published := publishedRunFixture()
	published.Parameters = published.Parameters[:2]
	store := &fakeReportRunStore{published: published}
	cipher := &fakeReportParameterCipher{err: errors.New("must not encrypt")}
	service := NewReportRunServiceWithDependencies(store, cipher)
	_, err := service.Create(t.Context(), 17, 9, requestbody.ReportRunCreateRequest{Parameters: map[string]json.RawMessage{"storeCode": json.RawMessage(`"S001"`)}})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if cipher.calls != 0 || store.command.Run.SensitiveParametersCipher != "" || store.command.Run.SensitiveParametersKeyVersion != "" {
		t.Fatalf("cipher calls=%d run=%#v", cipher.calls, store.command.Run)
	}
}

func TestReportRunServiceFingerprintUsesBusinessParametersAndRefreshNonce(t *testing.T) {
	published := publishedRunFixture()
	published.Parameters = published.Parameters[:2]
	store := &fakeReportRunStore{published: published}
	service := NewReportRunServiceWithDependencies(store, &fakeReportParameterCipher{})
	request := requestbody.ReportRunCreateRequest{Parameters: map[string]json.RawMessage{"storeCode": json.RawMessage(`"S001"`)}}
	if _, err := service.Create(t.Context(), 17, 9, request); err != nil {
		t.Fatalf("first Create() error = %v", err)
	}
	firstFingerprint := store.command.Run.ExecutionFingerprint
	firstUUID := store.command.Run.RunUUID
	if _, err := service.Create(t.Context(), 17, 9, request); err != nil {
		t.Fatalf("second Create() error = %v", err)
	}
	if store.command.Run.RunUUID == firstUUID {
		t.Fatal("two runs unexpectedly share a UUID")
	}
	if store.command.Run.ExecutionFingerprint != firstFingerprint {
		t.Fatalf("fingerprint changed with run UUID: %q != %q", store.command.Run.ExecutionFingerprint, firstFingerprint)
	}
	request.RefreshNonce = "refresh-2"
	if _, err := service.Create(t.Context(), 17, 9, request); err != nil {
		t.Fatalf("refreshed Create() error = %v", err)
	}
	if store.command.Run.ExecutionFingerprint == firstFingerprint {
		t.Fatal("refresh nonce did not change execution fingerprint")
	}
}

func TestReportRunServiceMapsAuthorizationAndContractRaces(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		want error
	}{
		{name: "denied", err: reportrepo.ErrReportActionDenied, want: ErrReportRunDenied},
		{name: "missing", err: reportrepo.ErrPublishedReportNotFound, want: ErrReportNotFound},
		{name: "contract changed", err: reportrepo.ErrDraftVersionConflict, want: ErrReportConflict},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := &fakeReportRunStore{published: publishedRunFixture(), err: test.err}
			service := NewReportRunServiceWithDependencies(store, &fakeReportParameterCipher{})
			_, err := service.Create(t.Context(), 17, 9, requestbody.ReportRunCreateRequest{})
			if !errors.Is(err, test.want) {
				t.Fatalf("Create() error = %v, want %v", err, test.want)
			}
		})
	}
}

type fakeReportRunStore struct {
	published *reportrepo.PublishedReport
	command   *reportrepo.CreateRunCommand
	err       error
}

func (store *fakeReportRunStore) FindPublishedReport(context.Context, uint, uint, string) (*reportrepo.PublishedReport, error) {
	return store.published, store.err
}
func (store *fakeReportRunStore) CreateRun(_ context.Context, _, _ uint, command *reportrepo.CreateRunCommand) error {
	store.command = command
	command.Run.ID = 31
	command.Run.CreatedAt = time.Now().UTC()
	return store.err
}

type fakeReportParameterCipher struct {
	version    string
	ciphertext string
	plaintext  string
	calls      int
	err        error
}

func (cipher *fakeReportParameterCipher) Encrypt(plaintext []byte) (string, string, error) {
	cipher.calls++
	cipher.plaintext = string(plaintext)
	return cipher.version, cipher.ciphertext, cipher.err
}

func publishedRunFixture() *reportrepo.PublishedReport {
	return &reportrepo.PublishedReport{
		Definition: model.ReportDefinition{BaseModel: model.BaseModel{ID: 9}, OwnerUserID: 17, Status: model.ReportDefinitionStatusActive},
		Version: model.ReportVersion{BaseModel: model.BaseModel{ID: 23}, DefinitionID: 9, Status: model.ReportVersionStatusPublished,
			ContractHash: strings.Repeat("a", 64), ProcedureSignatureHash: strings.Repeat("b", 64), ResultSchemaHash: strings.Repeat("c", 64)},
		Parameters: []model.ReportParameter{
			{ParameterCode: "runId", ProcedureArgName: "P_RUN_ID", Position: 1, Direction: "IN", LogicalType: "string", OracleType: "VARCHAR2", Cardinality: "SINGLE", Required: true, SystemInjected: true, NullPolicy: "TYPED_NULL"},
			{ParameterCode: "storeCode", ProcedureArgName: "P_STORE_CODE", Position: 2, Direction: "IN", LogicalType: "enum", OracleType: "VARCHAR2", Cardinality: "SINGLE", Required: true, AllowedValuesJSON: model.JSONText(`["S001","S002"]`), NullPolicy: "TYPED_NULL"},
			{ParameterCode: "secret", ProcedureArgName: "P_SECRET", Position: 3, Direction: "IN", LogicalType: "string", OracleType: "VARCHAR2", Cardinality: "SINGLE", Required: true, Sensitive: true, NullPolicy: "TYPED_NULL"},
		},
	}
}
