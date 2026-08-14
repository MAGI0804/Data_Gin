package reportrepo

import (
	"testing"

	"gin-biz-web-api/model"

	"gorm.io/gorm"
)

func TestReportColumnInsertValuesPreservesFalseConfiguration(t *testing.T) {
	values := reportColumnInsertValues(model.ReportColumn{
		Nullable: false, PreviewVisible: false, ExportVisible: false, ExportAllowed: false,
	})
	for _, field := range []string{"nullable", "preview_visible", "export_visible", "export_allowed"} {
		value, exists := values[field]
		if !exists || value != false {
			t.Fatalf("%s = %#v, exists = %t; want explicit false", field, value, exists)
		}
	}
}

func TestReportColumnInsertValuesCoversEveryPersistedField(t *testing.T) {
	statement := &gorm.Statement{DB: newDryRunDB(t)}
	if err := statement.Parse(&columnRecord{}); err != nil {
		t.Fatalf("parse report column schema: %v", err)
	}
	values := reportColumnInsertValues(model.ReportColumn{})
	for _, databaseName := range statement.Schema.DBNames {
		if databaseName == "id" {
			continue
		}
		if _, exists := values[databaseName]; !exists {
			t.Fatalf("insert values omit persisted field %q", databaseName)
		}
	}
	if len(values) != len(statement.Schema.DBNames)-1 {
		t.Fatalf("insert values contain %d fields, want %d", len(values), len(statement.Schema.DBNames)-1)
	}
}
