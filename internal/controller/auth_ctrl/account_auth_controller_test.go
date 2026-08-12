package auth_ctrl

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"gin-biz-web-api/internal/service/auth_svc"
	"gin-biz-web-api/pkg/phonecode"
	"gin-biz-web-api/pkg/responses"
)

type fakeAccountAuthService struct {
	session      *auth_svc.ConsoleSessionDTO
	profile      *auth_svc.ConsoleProfileDTO
	err          error
	phone        string
	purpose      phonecode.Purpose
	code         string
	account      string
	password     string
	current      string
	userID       uint
	passwordCall int
}

func (f *fakeAccountAuthService) LoginPassword(_ context.Context, account, password string) (*auth_svc.ConsoleSessionDTO, error) {
	f.account, f.password = account, password
	return f.session, f.err
}
func (f *fakeAccountAuthService) SendPhoneCode(_ context.Context, phone string, purpose phonecode.Purpose) error {
	f.phone, f.purpose = phone, purpose
	return f.err
}
func (f *fakeAccountAuthService) LoginPhoneCode(_ context.Context, phone, code string) (*auth_svc.ConsoleSessionDTO, error) {
	f.phone, f.code = phone, code
	return f.session, f.err
}
func (f *fakeAccountAuthService) ResetPassword(_ context.Context, phone, code, password string) error {
	f.phone, f.code, f.password = phone, code, password
	return f.err
}
func (f *fakeAccountAuthService) ChangePassword(_ context.Context, userID uint, currentPassword, newPassword string) error {
	f.userID, f.current, f.password = userID, currentPassword, newPassword
	return f.err
}
func (f *fakeAccountAuthService) Profile(_ context.Context, userID uint) (*auth_svc.ConsoleProfileDTO, error) {
	f.userID = userID
	return f.profile, f.err
}

func TestAccountAuthControllerRejectsMalformedPasswordLogin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	controller := NewAccountAuthController(nil)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/auth/login/password", bytes.NewBufferString(`{"account":"operator"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	controller.LoginPassword(ctx)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", recorder.Code)
	}
	assertSafeErrorEnvelope(t, recorder.Body.Bytes())
}

func TestAccountAuthControllerRejectsUnknownPhoneCodePurpose(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &fakeAccountAuthService{}
	controller := NewAccountAuthController(service)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/auth/phone-codes", bytes.NewBufferString(`{"phone":"13800138000","purpose":"unknown"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	controller.SendPhoneCode(ctx)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	if service.purpose != phonecode.Purpose("UNKNOWN") {
		t.Fatalf("purpose = %q", service.purpose)
	}
}

func TestAccountAuthControllerPasswordLoginReturnsSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	lastLogin := time.Date(2026, 8, 12, 8, 0, 0, 0, time.UTC)
	service := &fakeAccountAuthService{session: &auth_svc.ConsoleSessionDTO{Token: "signed-token", User: auth_svc.ConsoleAccountDTO{ID: 7, Account: "operator", Phone: "138****8000", LastLoginAt: &lastLogin}}}
	controller := NewAccountAuthController(service)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/auth/login/password", bytes.NewBufferString(`{"account":"operator","password":"secret-password"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	controller.LoginPassword(ctx)
	if recorder.Code != http.StatusOK || service.account != "operator" || service.password != "secret-password" {
		t.Fatalf("status=%d service=%#v", recorder.Code, service)
	}
	if !bytes.Contains(recorder.Body.Bytes(), []byte("signed-token")) || bytes.Contains(recorder.Body.Bytes(), []byte("secret-password")) {
		t.Fatalf("response = %s", recorder.Body.String())
	}
}

func TestWriteAccountAuthErrorUsesStableMessages(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
	}{
		{name: "credentials", err: auth_svc.ErrInvalidCredentials, status: http.StatusUnauthorized},
		{name: "weak password", err: auth_svc.ErrPasswordTooWeak, status: http.StatusUnprocessableEntity},
		{name: "cooldown", err: phonecode.ErrCooldown, status: http.StatusTooManyRequests},
		{name: "provider", err: context.DeadlineExceeded, status: http.StatusServiceUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			writeAccountAuthError(ctx, tt.err)
			if recorder.Code != tt.status {
				t.Fatalf("status = %d, want %d", recorder.Code, tt.status)
			}
			assertSafeErrorEnvelope(t, recorder.Body.Bytes())
		})
	}
}

func assertSafeErrorEnvelope(t *testing.T, body []byte) {
	t.Helper()
	var response responses.ResponseData
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Code == 0 || response.Msg == "" || response.Data != nil {
		t.Fatalf("unsafe error response: %#v", response)
	}
}
