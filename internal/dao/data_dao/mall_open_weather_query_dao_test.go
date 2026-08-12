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

	rows, err := NewMallDAO(db).ListOpenWeatherMallsAfterID(t.Context(), 17, 7, 51)
	if err != nil {
		t.Fatalf("ListOpenWeatherMallsAfterID() error=%v", err)
	}
	_ = rows
	for _, fragment := range []string{
		"`address_raw`",
		"`address_standardized`",
		"`weather_longitude`",
		"`weather_latitude`",
		"status = ?",
		"geocode_status = ?",
		"weather_enabled = ?",
		"weather_longitude BETWEEN ? AND ?",
		"weather_latitude BETWEEN ? AND ?",
		"id > ?",
		"ORDER BY id ASC",
		"LIMIT 51",
		"scope_user",
		"user_mall_scopes",
	} {
		if !strings.Contains(statement, fragment) {
			t.Fatalf("statement missing %q: %s", fragment, statement)
		}
	}
	for _, internalColumn := range []string{
		"contact_phone", "custom_fields_json", "source_reference", "`longitude`", "`latitude`",
		"`coordinate_system`", "geocode_level", "geocode_confidence",
	} {
		if strings.Contains(statement, internalColumn) {
			t.Fatalf("statement selects internal column %q: %s", internalColumn, statement)
		}
	}
}

func TestMallDAOCountOpenWeatherMallsUsesSamePublicFilters(t *testing.T) {
	t.Parallel()
	db := dryRunWeatherDAOTestDB(t)
	var statement string
	if err := db.Callback().Query().After("gorm:query").Register("test:capture_open_weather_malls_count_sql", func(tx *gorm.DB) {
		statement = tx.Statement.SQL.String()
	}); err != nil {
		t.Fatalf("register SQL capture callback: %v", err)
	}
	if _, err := NewMallDAO(db).CountOpenWeatherMalls(t.Context(), 17); err != nil {
		t.Fatalf("CountOpenWeatherMalls() error=%v", err)
	}
	for _, fragment := range []string{"SELECT count(*)", "status = ?", "geocode_status = ?", "weather_enabled = ?", "weather_longitude BETWEEN ? AND ?", "weather_latitude BETWEEN ? AND ?", "scope_user", "user_mall_scopes"} {
		if !strings.Contains(statement, fragment) {
			t.Fatalf("statement missing %q: %s", fragment, statement)
		}
	}
	if strings.Contains(statement, "id > ?") || strings.Contains(statement, "LIMIT") || strings.Contains(statement, "ORDER BY") {
		t.Fatalf("count statement contains page boundary: %s", statement)
	}
}
