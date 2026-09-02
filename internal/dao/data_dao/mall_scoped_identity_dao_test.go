package data_dao

import (
	"strings"
	"testing"

	"gorm.io/gorm"
)

func TestMallDAOListScopedIdentitiesUsesActorMallScope(t *testing.T) {
	t.Parallel()
	db := dryRunWeatherDAOTestDB(t)
	var statement string
	if err := db.Callback().Query().After("gorm:query").Register("test:capture_scoped_mall_identities_sql", func(tx *gorm.DB) {
		statement = tx.Statement.SQL.String()
	}); err != nil {
		t.Fatalf("register SQL capture callback: %v", err)
	}
	if _, err := NewMallDAO(db).ListScopedIdentitiesAfterID(t.Context(), 17, 8, 25); err != nil {
		t.Fatalf("ListScopedIdentitiesAfterID() error = %v", err)
	}
	for _, fragment := range []string{
		"SELECT `id`,`mall_code`,`name_cn`", "scope_user", "user_mall_scopes", "id > ?", "ORDER BY id ASC", "LIMIT 25",
	} {
		if !strings.Contains(statement, fragment) {
			t.Fatalf("statement missing %q: %s", fragment, statement)
		}
	}
}
