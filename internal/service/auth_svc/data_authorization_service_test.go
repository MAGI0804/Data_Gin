package auth_svc

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"gin-biz-web-api/internal/requests/auth_request"
	"gin-biz-web-api/model"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestNormalizeDataAuthorizationGrantEnforcesAllowlistAndExpiry(t *testing.T) {
	now := time.Date(2026, 7, 28, 8, 0, 0, 0, time.UTC)
	tests := []struct {
		name       string
		permission string
		expiresAt  string
		wantError  bool
	}{
		{name: "weather", permission: model.PermissionWeatherRead, expiresAt: now.Add(30 * 24 * time.Hour).Format(time.RFC3339)},
		{name: "bojun", permission: model.PermissionBojunOrderRead, expiresAt: now.Add(time.Hour).Format(time.RFC3339)},
		{name: "write permission rejected", permission: model.PermissionMallWrite, expiresAt: now.Add(time.Hour).Format(time.RFC3339), wantError: true},
		{name: "too soon", permission: model.PermissionWeatherRead, expiresAt: now.Add(time.Minute).Format(time.RFC3339), wantError: true},
		{name: "too long", permission: model.PermissionWeatherRead, expiresAt: now.Add(366 * 24 * time.Hour).Format(time.RFC3339), wantError: true},
		{name: "invalid timestamp", permission: model.PermissionWeatherRead, expiresAt: "2026-07-28", wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, _, err := normalizeDataAuthorizationGrant(tt.permission, tt.expiresAt, "业务接入", now)
			if (err != nil) != tt.wantError {
				t.Fatalf("error = %v, wantError %t", err, tt.wantError)
			}
		})
	}
}

func TestNormalizeDataAuthorizationCreateRejectsReservedAndDuplicatePermissions(t *testing.T) {
	now := time.Date(2026, 7, 28, 8, 0, 0, 0, time.UTC)
	service := &DataAuthorizationService{now: func() time.Time { return now }}
	valid := auth_request.DataAuthorizationAccountCreateRequest{
		Account: "partner_weather_01", Email: "owner@example.com", Nickname: "合作方账号", Reason: "天气数据接入",
		Permissions: []auth_request.DataAuthorizationPermissionInput{{Permission: model.PermissionWeatherRead, ExpiresAt: now.Add(30 * 24 * time.Hour).Format(time.RFC3339)}},
	}
	if _, grants, err := service.normalizeCreate(valid); err != nil || len(grants) != 1 {
		t.Fatalf("normalizeCreate(valid) grants=%v error=%v", grants, err)
	}
	reserved := valid
	reserved.Account = " Admin "
	if _, _, err := service.normalizeCreate(reserved); err == nil {
		t.Fatal("normalizeCreate() accepted reserved admin")
	}
	duplicate := valid
	duplicate.Permissions = append(duplicate.Permissions, duplicate.Permissions[0])
	if _, _, err := service.normalizeCreate(duplicate); err == nil {
		t.Fatal("normalizeCreate() accepted duplicate permission")
	}
}

func TestCreateDataAuthorizationUserOmitsNullablePhone(t *testing.T) {
	db, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      &sql.DB{},
		SkipInitializeWithVersion: true,
	}), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	db = db.Session(&gorm.Session{SkipDefaultTransaction: true})

	var insertSQL string
	if err := db.Callback().Create().After("gorm:create").Register("test:capture_open_api_user", func(tx *gorm.DB) {
		insertSQL = tx.Statement.SQL.String()
	}); err != nil {
		t.Fatalf("register SQL capture callback: %v", err)
	}
	user := &model.User{
		BaseModel:             &model.BaseModel{},
		CommonTimestampsField: &model.CommonTimestampsField{},
		Account:               "partner_weather_01",
		Email:                 "owner@example.com",
		Nickname:              "合作方账号",
		Password:              strings.Repeat("x", 60),
	}
	if err := createDataAuthorizationUser(context.Background(), db, user); err != nil {
		t.Fatalf("createDataAuthorizationUser() error = %v", err)
	}
	if insertSQL == "" || !strings.Contains(insertSQL, "`account`") {
		t.Fatalf("user INSERT was not captured: %q", insertSQL)
	}
	if strings.Contains(insertSQL, "`phone`") {
		t.Fatalf("user INSERT contains nullable phone zero value: %q", insertSQL)
	}
}

func TestDataAuthorizationReasonRejectsLogInjection(t *testing.T) {
	for _, reason := range []string{"", "line one\nline two", "carriage\rreturn", "nul\x00byte", strings.Repeat("理", 501)} {
		if _, err := normalizeDataAuthorizationReason(reason); err == nil {
			t.Fatalf("reason %q was accepted", reason)
		}
	}
}

func TestGenerateOpenAPITokenProducesOpaqueCredential(t *testing.T) {
	token, err := generateOpenAPIToken()
	if err != nil {
		t.Fatalf("generateOpenAPIToken() error = %v", err)
	}
	if !strings.HasPrefix(token, "dg_open_") || len(tokenDigest(token)) != 64 || tokenDisplayPrefix(token) == token {
		t.Fatalf("token shape is invalid")
	}
	data, err := json.Marshal(model.OpenAPICredential{TokenHash: tokenDigest(token), TokenPrefix: tokenDisplayPrefix(token)})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if strings.Contains(string(data), token) || strings.Contains(string(data), tokenDigest(token)) {
		t.Fatal("credential JSON leaked token or hash")
	}
}

func TestPermissionDTOStates(t *testing.T) {
	now := time.Now().UTC()
	future, past := now.Add(time.Hour), now.Add(-time.Hour)
	if got := permissionDTO(model.PermissionWeatherRead, nil, now).Status; got != "NOT_GRANTED" {
		t.Fatalf("nil status = %q", got)
	}
	if got := permissionDTO(model.PermissionWeatherRead, &future, now).Status; got != "ACTIVE" {
		t.Fatalf("future status = %q", got)
	}
	if got := permissionDTO(model.PermissionWeatherRead, &past, now).Status; got != "EXPIRED" {
		t.Fatalf("past status = %q", got)
	}
}
