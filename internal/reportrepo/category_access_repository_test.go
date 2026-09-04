package reportrepo

import (
	"strings"
	"testing"

	"gorm.io/gorm"
)

func TestCategoryAccessListKeepsConfiguredCategoriesWithoutReports(t *testing.T) {
	statement := buildCategoryAccessListQuery(newDryRunDB(t).Session(&gorm.Session{DryRun: true})).Scan(&[]categoryAccessRecord{})
	sqlText := statement.Statement.SQL.String()
	for _, fragment := range []string{
		"SELECT category FROM report_definitions WHERE TRIM(category) <> ''",
		"UNION",
		"SELECT category FROM report_category_access",
		"LEFT JOIN report_definitions AS definitions ON definitions.category = categories.category",
	} {
		if !strings.Contains(sqlText, fragment) {
			t.Fatalf("category access SQL %q does not contain %q", sqlText, fragment)
		}
	}
}
