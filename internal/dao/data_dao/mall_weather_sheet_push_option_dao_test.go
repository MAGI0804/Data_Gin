package data_dao

import (
	"strings"
	"testing"

	"gorm.io/gorm"
)

func TestMallWeatherSheetPushOptionDAOQueriesOnlyCompatibleRows(t *testing.T) {
	t.Parallel()

	db := dryRunWeatherDAOTestDB(t)
	statements := make([]string, 0, 2)
	if err := db.Callback().Query().After("gorm:query").Register("test:capture_sheet_push_options_sql", func(tx *gorm.DB) {
		statements = append(statements, tx.Statement.SQL.String())
	}); err != nil {
		t.Fatalf("register SQL capture callback: %v", err)
	}
	dao := NewMallWeatherSheetPushOptionDAO(db)
	destinations, err := dao.ListEnabledDestinations(t.Context(), "feishu_sheet", 201)
	if err != nil {
		t.Fatalf("ListEnabledDestinations() error=%v", err)
	}
	if destinations == nil {
		t.Fatal("ListEnabledDestinations() returned a nil slice")
	}
	if len(statements) != 1 {
		t.Fatalf("destination query statements=%v", statements)
	}
	destinationStatement := statements[0]
	for _, fragment := range []string{
		"SELECT `id`,`name`,`code`,`destination_type`,`config_json`,`enabled`",
		"enabled = ? AND destination_type = ?",
		"ORDER BY id ASC",
		"LIMIT 201",
	} {
		if !strings.Contains(destinationStatement, fragment) {
			t.Fatalf("destination statement missing %q: %s", fragment, destinationStatement)
		}
	}
	if strings.Contains(destinationStatement, "feishu_sheet") {
		t.Fatalf("destination statement interpolated type: %s", destinationStatement)
	}

	profiles, err := dao.ListEnabledProfilesByCodes(t.Context(), []string{"mall_weather_full"})
	if err != nil {
		t.Fatalf("ListEnabledProfilesByCodes() error=%v", err)
	}
	if profiles == nil {
		t.Fatal("ListEnabledProfilesByCodes() returned a nil slice")
	}
	if len(statements) != 2 {
		t.Fatalf("profile query statements=%v", statements)
	}
	profileStatement := statements[1]
	for _, fragment := range []string{
		"SELECT `id`,`code`,`version`,`enabled`",
		"enabled = ? AND code IN (?)",
		"ORDER BY code ASC",
	} {
		if !strings.Contains(profileStatement, fragment) {
			t.Fatalf("profile statement missing %q: %s", fragment, profileStatement)
		}
	}
	if strings.Contains(profileStatement, "mall_weather_full") {
		t.Fatalf("profile statement interpolated code: %s", profileStatement)
	}
}

func TestMallWeatherSheetPushOptionDAORejectsInvalidInputs(t *testing.T) {
	t.Parallel()

	dao := &MallWeatherSheetPushOptionDAO{}
	if _, err := dao.ListEnabledDestinations(t.Context(), "feishu_sheet", 200); err == nil {
		t.Fatal("ListEnabledDestinations() accepted an unconfigured DAO")
	}
	configured := NewMallWeatherSheetPushOptionDAO(dryRunWeatherDAOTestDB(t))
	if _, err := configured.ListEnabledDestinations(t.Context(), "", 200); err == nil {
		t.Fatal("ListEnabledDestinations() accepted an empty type")
	}
	if _, err := configured.ListEnabledDestinations(t.Context(), "feishu_sheet", 202); err == nil {
		t.Fatal("ListEnabledDestinations() accepted an unbounded limit")
	}
	if _, err := configured.ListEnabledProfilesByCodes(t.Context(), nil); err == nil {
		t.Fatal("ListEnabledProfilesByCodes() accepted empty codes")
	}
}
