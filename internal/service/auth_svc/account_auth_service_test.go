package auth_svc

import (
	"context"
	"errors"
	"testing"
	"time"

	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/phonecode"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type fakeAccountAuthRepository struct {
	byAccount     *consoleAccount
	byPhone       *consoleAccount
	byID          *consoleAccount
	profile       *consoleProfile
	findErr       error
	updateErr     error
	loginAt       time.Time
	passwordAt    time.Time
	password      string
	accountLookup string
	phoneLookup   string
	normalizedID  uint
}

func (r *fakeAccountAuthRepository) FindActiveConsoleByAccount(_ context.Context, account string) (*consoleAccount, error) {
	r.accountLookup = account
	return r.byAccount, r.findErr
}
func (r *fakeAccountAuthRepository) FindActiveConsoleByPhone(_ context.Context, phone string) (*consoleAccount, error) {
	r.phoneLookup = phone
	return r.byPhone, r.findErr
}
func (r *fakeAccountAuthRepository) FindActiveConsoleByID(context.Context, uint) (*consoleAccount, error) {
	return r.byID, r.findErr
}
func (r *fakeAccountAuthRepository) RecordLogin(_ context.Context, _ uint, at time.Time) (*consoleAccount, error) {
	r.loginAt = at
	if r.findErr != nil {
		return nil, r.findErr
	}
	return r.byID, nil
}
func (r *fakeAccountAuthRepository) UpdatePassword(_ context.Context, _ uint, passwordHash string, at time.Time) error {
	r.password, r.passwordAt = passwordHash, at
	return r.updateErr
}
func (r *fakeAccountAuthRepository) LoadProfile(context.Context, uint) (*consoleProfile, error) {
	return r.profile, r.findErr
}
func (r *fakeAccountAuthRepository) NormalizeConsoleAdminAccess(_ context.Context, userID uint) error {
	r.normalizedID = userID
	return r.updateErr
}

type fakePhoneCodes struct {
	issuedPhone   string
	issuedPurpose phonecode.Purpose
	verifiedCode  string
	err           error
}

func (f *fakePhoneCodes) Issue(_ context.Context, purpose phonecode.Purpose, phone string) error {
	f.issuedPhone, f.issuedPurpose = phone, purpose
	return f.err
}
func (f *fakePhoneCodes) VerifyAndConsume(_ context.Context, purpose phonecode.Purpose, phone, code string) error {
	f.issuedPhone, f.issuedPurpose, f.verifiedCode = phone, purpose, code
	return f.err
}

type fakeTokenIssuer struct {
	userID      string
	authVersion uint64
}

func (f *fakeTokenIssuer) GenerateVersionedToken(userID, _ string, authVersion uint64) string {
	f.userID, f.authVersion = userID, authVersion
	return "versioned-token"
}

func testConsoleAccount(t *testing.T) *consoleAccount {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	phone := "13800138000"
	return &consoleAccount{ID: 7, Account: "operator", Phone: &phone, Password: string(hash), Nickname: "运营", AccountType: model.AccountTypeConsole, Status: model.AccountStatusActive, AuthVersion: 9, MallScopeMode: model.MallScopeSelected}
}

func TestAccountAuthServicePasswordLoginWorksWithoutPhoneCodeProvider(t *testing.T) {
	account := testConsoleAccount(t)
	repository := &fakeAccountAuthRepository{byAccount: account, byID: account}
	tokens := &fakeTokenIssuer{}
	service := NewAccountAuthService(repository, nil, tokens)
	now := time.Date(2026, 8, 12, 8, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }

	session, err := service.LoginPassword(context.Background(), "operator", "correct-password")
	if err != nil {
		t.Fatalf("LoginPassword() error = %v", err)
	}
	if session.Token != "versioned-token" || session.User.Phone != "138****8000" {
		t.Fatalf("session = %#v", session)
	}
	if tokens.userID != "7" || tokens.authVersion != 9 {
		t.Fatalf("token args = %q/%d", tokens.userID, tokens.authVersion)
	}
	if repository.normalizedID != 7 {
		t.Fatalf("normalized user id = %d", repository.normalizedID)
	}
	if !repository.loginAt.Equal(now) {
		t.Fatalf("last login = %v, want %v", repository.loginAt, now)
	}
}

func TestAccountAuthServicePasswordLoginAcceptsPhone(t *testing.T) {
	account := testConsoleAccount(t)
	repository := &fakeAccountAuthRepository{byPhone: account, byID: account}
	service := NewAccountAuthService(repository, nil, &fakeTokenIssuer{})
	if _, err := service.LoginPassword(context.Background(), "13800138000", "correct-password"); err != nil {
		t.Fatalf("LoginPassword(phone) error = %v", err)
	}
	if repository.phoneLookup != "13800138000" || repository.accountLookup != "" {
		t.Fatalf("lookups phone=%q account=%q", repository.phoneLookup, repository.accountLookup)
	}
}

func TestAccountAuthServicePasswordLoginRejectsInvalidPassword(t *testing.T) {
	account := testConsoleAccount(t)
	service := NewAccountAuthService(&fakeAccountAuthRepository{byAccount: account}, nil, &fakeTokenIssuer{})
	if _, err := service.LoginPassword(context.Background(), "operator", "wrong"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("LoginPassword() error = %v", err)
	}
}

func TestAccountAuthServiceSendPasswordResetCodeDoesNotEnumerateAccounts(t *testing.T) {
	codes := &fakePhoneCodes{}
	service := NewAccountAuthService(&fakeAccountAuthRepository{findErr: gorm.ErrRecordNotFound}, codes, &fakeTokenIssuer{})
	if err := service.SendPhoneCode(context.Background(), "13800138000", phonecode.PurposePasswordReset); err != nil {
		t.Fatalf("SendPhoneCode() error = %v", err)
	}
	if codes.issuedPhone != "" {
		t.Fatal("unknown account caused an SMS send")
	}
}

func TestAccountAuthServicePhoneLoginConsumesPurposeAndVersionsToken(t *testing.T) {
	account := testConsoleAccount(t)
	repository := &fakeAccountAuthRepository{byPhone: account, byID: account}
	codes, tokens := &fakePhoneCodes{}, &fakeTokenIssuer{}
	service := NewAccountAuthService(repository, codes, tokens)
	session, err := service.LoginPhoneCode(context.Background(), "13800138000", "123456")
	if err != nil {
		t.Fatalf("LoginPhoneCode() error = %v", err)
	}
	if session.Token == "" || codes.issuedPurpose != phonecode.PurposeLogin || codes.verifiedCode != "123456" || tokens.authVersion != 9 {
		t.Fatalf("unexpected login result: session=%#v codes=%#v tokens=%#v", session, codes, tokens)
	}
}

func TestAccountAuthServicePasswordChangesHashAndUpdateVersion(t *testing.T) {
	account := testConsoleAccount(t)
	repository := &fakeAccountAuthRepository{byPhone: account, byID: account}
	codes := &fakePhoneCodes{}
	service := NewAccountAuthService(repository, codes, &fakeTokenIssuer{})

	if err := service.ResetPassword(context.Background(), "13800138000", "123456", "new-password-123"); err != nil {
		t.Fatalf("ResetPassword() error = %v", err)
	}
	if codes.issuedPurpose != phonecode.PurposePasswordReset || bcrypt.CompareHashAndPassword([]byte(repository.password), []byte("new-password-123")) != nil {
		t.Fatal("ResetPassword() did not verify code and persist a bcrypt hash")
	}

	repository.password = ""
	if err := service.ChangePassword(context.Background(), 7, "correct-password", "different-password-123"); err != nil {
		t.Fatalf("ChangePassword() error = %v", err)
	}
	if bcrypt.CompareHashAndPassword([]byte(repository.password), []byte("different-password-123")) != nil {
		t.Fatal("ChangePassword() did not persist a bcrypt hash")
	}
}

func TestAccountAuthServiceProfileSortsAndDoesNotExposeSecrets(t *testing.T) {
	account := *testConsoleAccount(t)
	repository := &fakeAccountAuthRepository{profile: &consoleProfile{Account: account, Roles: []ConsoleRoleDTO{{Code: "viewer"}, {Code: "admin"}}, Permissions: []string{"z.read", "a.read"}, MallIDs: []uint{9, 2}}}
	service := NewAccountAuthService(repository, nil, &fakeTokenIssuer{})
	profile, err := service.Profile(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if profile.Roles[0].Code != "admin" || profile.Permissions[0] != "a.read" || profile.MallIDs[0] != 2 || profile.Phone != "138****8000" {
		t.Fatalf("profile = %#v", profile)
	}
}
