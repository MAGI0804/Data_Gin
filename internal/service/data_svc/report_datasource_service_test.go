package data_svc

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"gin-biz-web-api/internal/reportoracle"
	"gin-biz-web-api/internal/reportrepo"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/model"
)

func TestReportDatasourceServiceCreateEncryptsAndRedactsPassword(t *testing.T) {
	store := &fakeReportDatasourceStore{}
	cipher := &fakeReportDatasourceCipher{}
	service := NewReportDatasourceServiceWithDependencies(store, cipher, func(context.Context, reportoracle.Config) (reportDatasourceConnection, error) { return nil, nil })
	result, err := service.Create(t.Context(), 7, validReportDatasourceRequest("secret-password"))
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if cipher.encrypted != "secret-password" || store.created == nil || store.created.PasswordCiphertext != "ciphertext" || store.created.CredentialKeyVersion != "key-v1" {
		t.Fatalf("credential was not encrypted before persistence: cipher=%q datasource=%+v", cipher.encrypted, store.created)
	}
	if !result.HasPassword || strings.Contains(result.Host+result.Username+result.LastTestError, "secret-password") {
		t.Fatalf("Create() leaked or lost password state: %+v", result)
	}
}

func TestReportDatasourceServiceUpdateKeepsCredentialWhenPasswordOmitted(t *testing.T) {
	store := &fakeReportDatasourceStore{item: model.ReportDatasource{BaseModel: model.BaseModel{ID: 4}, Code: "report_oracle", Driver: model.ReportDatasourceDriverOracle, PasswordCiphertext: "existing", CredentialKeyVersion: "old"}}
	cipher := &fakeReportDatasourceCipher{}
	service := NewReportDatasourceServiceWithDependencies(store, cipher, func(context.Context, reportoracle.Config) (reportDatasourceConnection, error) { return nil, nil })
	request := validReportDatasourceRequest("")
	result, err := service.Update(t.Context(), 7, 4, request)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if cipher.encrypted != "" || store.updated == nil || store.updated.PasswordCiphertext != "" || !result.HasPassword {
		t.Fatalf("Update() unexpectedly rotated credential: cipher=%q updated=%+v result=%+v", cipher.encrypted, store.updated, result)
	}
}

func TestReportDatasourceServiceTestReturnsOnlySafeFailure(t *testing.T) {
	store := &fakeReportDatasourceStore{item: model.ReportDatasource{BaseModel: model.BaseModel{ID: 4}, Driver: model.ReportDatasourceDriverOracle, PasswordCiphertext: "ciphertext", CredentialKeyVersion: "key-v1", ConnectTimeoutSeconds: 5}}
	service := NewReportDatasourceServiceWithDependencies(store, &fakeReportDatasourceCipher{}, func(context.Context, reportoracle.Config) (reportDatasourceConnection, error) {
		return nil, errors.New("ORA-01017 password=secret-password host=private.example")
	})
	start := time.Date(2026, 8, 13, 1, 2, 3, 0, time.UTC)
	service.now = func() time.Time { start = start.Add(25 * time.Millisecond); return start }
	result, err := service.Test(t.Context(), 7, 4)
	if err != nil {
		t.Fatalf("Test() error = %v", err)
	}
	if result.Status != reportDatasourceTestFailed || result.ErrorCode != "AUTHENTICATION_FAILED" || strings.Contains(result.Message+store.testError, "secret-password") || store.testStatus != reportDatasourceTestFailed {
		t.Fatalf("Test() result = %+v, stored=%q/%q", result, store.testStatus, store.testError)
	}
}

func TestReportDatasourceConnectionFailureClassifiesSafeOperationalErrors(t *testing.T) {
	for _, test := range []struct {
		name     string
		err      error
		wantCode string
		wantText string
	}{
		{name: "missing instant client", err: errors.New("DPI-1047: cannot locate a 64-bit Oracle Client library: password=secret"), wantCode: "ORACLE_CLIENT_UNAVAILABLE", wantText: "服务端 Oracle 客户端不可用，请联系管理员"},
		{name: "locked account", err: errors.New("ORA-28000: the account is locked"), wantCode: "ACCOUNT_LOCKED", wantText: "Oracle 账号已锁定"},
		{name: "expired password", err: errors.New("ORA-28001: the password has expired"), wantCode: "PASSWORD_EXPIRED", wantText: "Oracle 密码已过期"},
		{name: "unknown sid", err: errors.New("ORA-12505: listener does not currently know of SID"), wantCode: "SERVICE_NOT_FOUND", wantText: "Oracle 服务名或 SID 不可用"},
		{name: "unreachable network", err: errors.New("dial tcp: connect: network is unreachable"), wantCode: "NETWORK_UNREACHABLE", wantText: "无法连接 Oracle 网络地址"},
	} {
		t.Run(test.name, func(t *testing.T) {
			code, message := safeDatasourceConnectionFailure(test.err)
			if code != test.wantCode || message != test.wantText || strings.Contains(message, "secret") {
				t.Fatalf("safeDatasourceConnectionFailure() = %q, %q", code, message)
			}
		})
	}
}

func TestReportDatasourceServiceTestsUnsavedConnectionDraft(t *testing.T) {
	store := &fakeReportDatasourceStore{}
	var opened reportoracle.Config
	connection := &fakeReportDatasourceConnection{}
	service := NewReportDatasourceServiceWithDependencies(store, &fakeReportDatasourceCipher{}, func(_ context.Context, config reportoracle.Config) (reportDatasourceConnection, error) {
		opened = config
		return connection, nil
	})
	request := validReportDatasourceConnectionTestRequest("draft-password")
	result, err := service.TestConnection(t.Context(), 7, request)
	if err != nil {
		t.Fatalf("TestConnection() error = %v", err)
	}
	if result.Status != reportDatasourceTestSuccess || opened.Host != "draft.oracle.internal" || opened.ServiceName != "DRAFT" || opened.Username != "draft_user" || opened.Password != "draft-password" {
		t.Fatalf("TestConnection() result=%+v config=%+v", result, opened)
	}
	if store.created != nil || store.updated != nil || store.testStatus != "" {
		t.Fatalf("unsaved draft test mutated MySQL store: %+v", store)
	}
	if !connection.closed {
		t.Fatal("successful draft connection was not closed")
	}
}

func TestReportDatasourceServiceConnectionDraftClosesConnectionAndMapsTimeout(t *testing.T) {
	connection := &fakeReportDatasourceConnection{}
	service := NewReportDatasourceServiceWithDependencies(&fakeReportDatasourceStore{}, &fakeReportDatasourceCipher{}, func(context.Context, reportoracle.Config) (reportDatasourceConnection, error) {
		return connection, context.DeadlineExceeded
	})
	result, err := service.TestConnection(t.Context(), 7, validReportDatasourceConnectionTestRequest("draft-password"))
	if err != nil {
		t.Fatalf("TestConnection() error = %v", err)
	}
	if result.Status != reportDatasourceTestFailed || result.ErrorCode != "CONNECT_TIMEOUT" || !connection.closed {
		t.Fatalf("TestConnection() result=%+v closed=%t", result, connection.closed)
	}
}

func TestReportDatasourceServiceConnectionDraftReusesExistingPassword(t *testing.T) {
	store := &fakeReportDatasourceStore{item: model.ReportDatasource{BaseModel: model.BaseModel{ID: 4}, PasswordCiphertext: "ciphertext", CredentialKeyVersion: "key-v1"}}
	var opened reportoracle.Config
	service := NewReportDatasourceServiceWithDependencies(store, &fakeReportDatasourceCipher{}, func(_ context.Context, config reportoracle.Config) (reportDatasourceConnection, error) {
		opened = config
		return &fakeReportDatasourceConnection{}, nil
	})
	request := validReportDatasourceConnectionTestRequest("")
	request.DatasourceID = 4
	result, err := service.TestConnection(t.Context(), 7, request)
	if err != nil || result.Status != reportDatasourceTestSuccess || opened.Password != "secret-password" || opened.Host != "draft.oracle.internal" {
		t.Fatalf("TestConnection() result=%+v error=%v config=%+v", result, err, opened)
	}
}

func TestReportDatasourceServiceConnectionDraftRequiresPasswordOrDatasource(t *testing.T) {
	service := NewReportDatasourceServiceWithDependencies(&fakeReportDatasourceStore{}, &fakeReportDatasourceCipher{}, func(context.Context, reportoracle.Config) (reportDatasourceConnection, error) { return nil, nil })
	if _, err := service.TestConnection(t.Context(), 7, validReportDatasourceConnectionTestRequest("")); !errors.Is(err, ErrReportDatasourceInvalid) {
		t.Fatalf("TestConnection() error = %v, want ErrReportDatasourceInvalid", err)
	}
}

func TestReportDatasourceRequestRequiresExactlyOneOracleServiceSelector(t *testing.T) {
	for _, test := range []struct {
		name        string
		serviceName string
		sid         string
		valid       bool
	}{{name: "service name", serviceName: "REPORT", valid: true}, {name: "sid", sid: "REPORT", valid: true}, {name: "missing", valid: false}, {name: "both", serviceName: "REPORT", sid: "REPORT", valid: false}} {
		t.Run(test.name, func(t *testing.T) {
			request := validReportDatasourceRequest("password")
			request.ServiceName, request.SID = test.serviceName, test.sid
			_, err := reportDatasourceFromRequest(request, true)
			if (err == nil) != test.valid {
				t.Fatalf("reportDatasourceFromRequest() error = %v, valid=%t", err, test.valid)
			}
		})
	}
}

func TestReportDatasourceWhitespacePasswordPolicyIsExplicit(t *testing.T) {
	request := validReportDatasourceRequest("   ")
	if _, err := reportDatasourceFromRequest(request, true); err != nil {
		t.Fatalf("Oracle passwords containing only spaces are valid credentials and must not be trimmed: %v", err)
	}
	store := &fakeReportDatasourceStore{item: model.ReportDatasource{BaseModel: model.BaseModel{ID: 4}, Driver: model.ReportDatasourceDriverOracle, PasswordCiphertext: "existing", CredentialKeyVersion: "old"}}
	cipher := &fakeReportDatasourceCipher{}
	service := NewReportDatasourceServiceWithDependencies(store, cipher, func(context.Context, reportoracle.Config) (reportDatasourceConnection, error) { return nil, nil })
	if _, err := service.Update(t.Context(), 7, 4, request); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if cipher.encrypted != "   " || store.updated == nil || store.updated.PasswordCiphertext == "" {
		t.Fatalf("space-only Oracle password was not intentionally rotated: cipher=%q updated=%+v", cipher.encrypted, store.updated)
	}
}

type fakeReportDatasourceStore struct {
	item       model.ReportDatasource
	created    *model.ReportDatasource
	updated    *model.ReportDatasource
	testStatus string
	testError  string
}

func (store *fakeReportDatasourceStore) ListReportDatasources(context.Context) ([]model.ReportDatasource, error) {
	return []model.ReportDatasource{store.item}, nil
}
func (store *fakeReportDatasourceStore) GetReportDatasource(_ context.Context, id uint) (*model.ReportDatasource, error) {
	if id == 0 {
		return nil, reportrepo.ErrDatasourceNotFound
	}
	copy := store.item
	copy.ID = id
	return &copy, nil
}
func (store *fakeReportDatasourceStore) CreateReportDatasource(_ context.Context, _ uint, datasource *model.ReportDatasource) error {
	copy := *datasource
	copy.ID = 4
	store.created = &copy
	datasource.ID = 4
	store.item = copy
	return nil
}
func (store *fakeReportDatasourceStore) UpdateReportDatasource(_ context.Context, _ uint, datasource *model.ReportDatasource) error {
	copy := *datasource
	store.updated = &copy
	if datasource.PasswordCiphertext != "" {
		store.item.PasswordCiphertext = datasource.PasswordCiphertext
		store.item.CredentialKeyVersion = datasource.CredentialKeyVersion
	}
	store.item.BaseModel = datasource.BaseModel
	store.item.Code = datasource.Code
	store.item.Name = datasource.Name
	store.item.Driver = datasource.Driver
	return nil
}
func (store *fakeReportDatasourceStore) RecordReportDatasourceTest(_ context.Context, _, _ uint, status, safeError string, _ time.Time) error {
	store.testStatus, store.testError = status, safeError
	return nil
}

type fakeReportDatasourceCipher struct{ encrypted string }

func (cipher *fakeReportDatasourceCipher) Encrypt(plaintext string) (string, string, error) {
	cipher.encrypted = plaintext
	return "key-v1", "ciphertext", nil
}
func (*fakeReportDatasourceCipher) Decrypt(string, string) (string, error) {
	return "secret-password", nil
}

func validReportDatasourceRequest(password string) requestbody.ReportDatasourceSaveRequest {
	return requestbody.ReportDatasourceSaveRequest{Code: "report_oracle", Name: "报表 Oracle", Host: "oracle.internal", Port: 1521, ServiceName: "REPORT", Username: "report_user", Password: password, SessionTimezone: "Asia/Shanghai", ConnectTimeoutSeconds: 5, QueryTimeoutSeconds: 300, MaxOpenConnections: 10, MaxIdleConnections: 2, PrefetchRows: 1000, ArraySize: 1000, Enabled: true}
}

type fakeReportDatasourceConnection struct{ closed bool }

func (connection *fakeReportDatasourceConnection) Close() error {
	connection.closed = true
	return nil
}

func validReportDatasourceConnectionTestRequest(password string) requestbody.ReportDatasourceConnectionTestRequest {
	return requestbody.ReportDatasourceConnectionTestRequest{Host: "draft.oracle.internal", Port: 1521, ServiceName: "DRAFT", Username: "draft_user", Password: password, SessionTimezone: "Asia/Shanghai", ConnectTimeoutSeconds: 5, QueryTimeoutSeconds: 300, MaxOpenConnections: 10, MaxIdleConnections: 2, PrefetchRows: 1000, ArraySize: 1000}
}
