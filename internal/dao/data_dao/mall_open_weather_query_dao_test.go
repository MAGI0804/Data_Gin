package data_dao

import (
	"strings"
	"testing"

	"gorm.io/gorm"
)

func TestMallDAOListOpenWeatherMallsUsesBoundedPublicQuery(t *testing.T) {
	t.Parallel()
	db := dryRunWeatherDAOTestDB(t)
	var statement string
	if err := db.Callback().Query().After("gorm:query").Register("test:capture_open_weather_malls_sql", func(tx *gorm.DB) {
		statement = tx.Statement.SQL.String()
	}); err != nil {
		t.Fatalf("register SQL capture callback: %v", err)
	}

	rows, err := NewMallDAO(db).ListOpenWeatherMallsAfterID(t.Context(), 7, 51)
	if err != nil {
		t.Fatalf("ListOpenWeatherMallsAfterID() error=%v", err)
	}
	_ = rows
	for _, fragment := range []string{
		"SELECT `id`,`mall_code`,`name_cn`,`name_en`,`province`,`city`,`district`,`timezone`,`weather_enabled`",
		"status = ?",
		"geocode_status = ?",
		"weather_enabled = ?",
		"weather_longitude BETWEEN ? AND ?",
		"weather_latitude BETWEEN ? AND ?",
		"id > ?",
		"ORDER BY id ASC",
		"LIMIT 51",
	} {
		if !strings.Contains(statement, fragment) {
			t.Fatalf("statement missing %q: %s", fragment, statement)
		}
	}
	for _, internalColumn := range []string{"contact_phone", "custom_fields_json", "source_reference"} {
		if strings.Contains(statement, internalColumn) {
			t.Fatalf("statement selects internal column %q: %s", internalColumn, statement)
		}
	}
}
