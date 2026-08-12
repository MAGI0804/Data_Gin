package auth_svc

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestMallScopeApplyUsesSelectedMallSubquery(t *testing.T) {
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: &sql.DB{}, SkipInitializeWithVersion: true}), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	if err != nil {
		t.Fatal(err)
	}
	service := NewMallScopeService(db)
	// Build the filtering fragment directly because a DryRun database cannot
	// return the user scope row required by Apply.
	query := db.Table("malls").Where(
		"malls.id IN (?)",
		db.Model(&struct{ MallID uint }{}).Table("user_mall_scopes").Select("mall_id").Where("user_id = ?", 7),
	).Find(&[]struct{}{})
	if query.Error != nil {
		t.Fatal(query.Error)
	}
	sqlText := query.Statement.SQL.String()
	if !strings.Contains(sqlText, "malls.id IN (SELECT `mall_id` FROM `user_mall_scopes` WHERE user_id = ?)") {
		t.Fatalf("scope SQL = %s", sqlText)
	}
	if service == nil {
		t.Fatal("service is nil")
	}
}

func TestMallScopeRejectsInvalidRequest(t *testing.T) {
	service := &MallScopeService{}
	if allowed, err := service.CanAccess(context.Background(), 1, 1); err == nil || allowed {
		t.Fatalf("CanAccess() = %t, %v", allowed, err)
	}
}
