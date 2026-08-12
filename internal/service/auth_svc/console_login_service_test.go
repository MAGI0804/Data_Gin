package auth_svc

import (
	"context"
	"database/sql"
	"testing"

	"gin-biz-web-api/model"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestConsoleAdminInitialPasswordComesFromEnvironment(t *testing.T) {
	t.Setenv(EnvConsoleAdminInitialPassword, "environment-secret")
	if value, ok := configuredInitialAdminPassword("environment-secret"); !ok || value != "environment-secret" {
		t.Fatalf("configuredInitialAdminPassword() = %q, %t", value, ok)
	}
	if _, ok := configuredInitialAdminPassword("old-hardcoded-password"); ok {
		t.Fatal("configuredInitialAdminPassword accepted a different password")
	}
}

func TestConsoleLoginRejectsInvalidCredentialsBeforeDatabase(t *testing.T) {
	service := &ConsoleLoginService{}
	if token, user, err := service.Login(context.Background(), "user", "wrong"); err == nil || token != "" || user != nil {
		t.Fatalf("Login() = token %q, user %#v, error %v", token, user, err)
	}
}

func TestGrantConsoleAdminPermissionsUsesCanonicalPermissionSet(t *testing.T) {
	db, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      &sql.DB{},
		SkipInitializeWithVersion: true,
	}), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	db = db.Session(&gorm.Session{SkipDefaultTransaction: true})

	seen := make(map[string]struct{})
	if err := db.Callback().Create().After("gorm:create").Register("test:capture_permissions", func(tx *gorm.DB) {
		for _, value := range tx.Statement.Vars {
			permission, ok := value.(string)
			if ok {
				seen[permission] = struct{}{}
			}
		}
	}); err != nil {
		t.Fatalf("register permission capture callback: %v", err)
	}

	if err := grantConsoleAdminPermissions(context.Background(), db, 19); err != nil {
		t.Fatalf("grantConsoleAdminPermissions() error = %v", err)
	}
	for _, permission := range model.MallWeatherAdminPermissions() {
		if _, exists := seen[permission]; !exists {
			t.Fatalf("grant missing canonical permission %q", permission)
		}
	}
}

func TestSyncExistingConsoleAdminPermissionsRejectsInvalidDatabase(t *testing.T) {
	if _, err := SyncExistingConsoleAdminPermissions(context.Background(), nil); err == nil {
		t.Fatal("SyncExistingConsoleAdminPermissions(nil) error = nil")
	}
}

func TestIsTrustedConsoleAdmin(t *testing.T) {
	adminEmail := "admin@warehouse.local"
	attackerEmail := "attacker@example.com"
	tests := []struct {
		name string
		user *model.User
		want bool
	}{
		{name: "console managed", user: &model.User{Account: "admin", Email: &adminEmail, Nickname: "管理员", ConsoleManaged: true}, want: true},
		{name: "new console managed without email", user: &model.User{Account: "admin", Nickname: "管理员", ConsoleManaged: true}, want: true},
		{name: "legacy console shape without marker", user: &model.User{Account: "admin", Email: &adminEmail, Nickname: "管理员"}},
		{name: "public registration shape", user: &model.User{Account: "admin", Email: &attackerEmail, ConsoleManaged: true}},
		{name: "case variant", user: &model.User{Account: "Admin", Email: &adminEmail, Nickname: "管理员", ConsoleManaged: true}},
		{name: "missing user"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isTrustedConsoleAdmin(tt.user); got != tt.want {
				t.Fatalf("isTrustedConsoleAdmin() = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestIsLegacyConsoleAdmin(t *testing.T) {
	email := "admin@warehouse.local"
	legacy := &model.User{Account: "admin", Email: &email, Nickname: "管理员"}
	if !isLegacyConsoleAdmin(legacy) {
		t.Fatal("isLegacyConsoleAdmin() = false")
	}
	legacy.Nickname = ""
	if isLegacyConsoleAdmin(legacy) {
		t.Fatal("isLegacyConsoleAdmin(public registration shape) = true")
	}
}

func TestIsDuplicateEntry(t *testing.T) {
	if !isDuplicateEntry(&mysqlDriver.MySQLError{Number: 1062, Message: "duplicate"}) {
		t.Fatal("isDuplicateEntry(1062) = false")
	}
	if isDuplicateEntry(&mysqlDriver.MySQLError{Number: 1048, Message: "not null"}) {
		t.Fatal("isDuplicateEntry(1048) = true")
	}
}
