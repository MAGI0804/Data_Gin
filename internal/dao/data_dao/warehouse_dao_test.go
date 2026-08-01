package data_dao

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"gin-biz-web-api/model"
)

func TestRawRecordOriginCondition(t *testing.T) {
	tests := []struct {
		name      string
		origin    string
		condition string
		argument  string
	}{
		{name: "pull", origin: "pull", condition: "metadata_json LIKE ?", argument: "%\"format\":\"fetch\"%"},
		{name: "receive", origin: "receive", condition: "(metadata_json IS NULL OR metadata_json NOT LIKE ?)", argument: "%\"format\":\"fetch\"%"},
		{name: "empty", origin: "", condition: "", argument: ""},
		{name: "unknown", origin: "unknown", condition: "", argument: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			condition, args := rawRecordOriginCondition(tt.origin)
			if condition != tt.condition {
				t.Fatalf("condition = %q, want %q", condition, tt.condition)
			}
			if tt.argument == "" {
				if len(args) != 0 {
					t.Fatalf("args = %#v, want none", args)
				}
				return
			}
			if len(args) != 1 || args[0] != tt.argument {
				t.Fatalf("args = %#v, want %q", args, tt.argument)
			}
		})
	}
}

func TestRawRecordQueryIndexes(t *testing.T) {
	typeOfRecord := reflect.TypeOf(model.RawRecord{})
	for _, field := range []struct {
		name       string
		indexNames []string
	}{
		{name: "SourceCode", indexNames: []string{"idx_raw_records_source_status_received"}},
		{name: "Status", indexNames: []string{"idx_raw_records_source_status_received", "idx_raw_records_status_received"}},
		{name: "ReceivedAt", indexNames: []string{"idx_raw_records_source_status_received", "idx_raw_records_status_received", "idx_raw_records_received_at"}},
	} {
		t.Run(field.name, func(t *testing.T) {
			structField, ok := typeOfRecord.FieldByName(field.name)
			if !ok {
				t.Fatalf("RawRecord has no %s field", field.name)
			}
			gormTag := structField.Tag.Get("gorm")
			for _, indexName := range field.indexNames {
				if !strings.Contains(gormTag, indexName) {
					t.Fatalf("gorm tag %q does not declare %s", gormTag, indexName)
				}
			}
		})
	}
}

func TestRawRecordListColumnsExcludeSensitiveFields(t *testing.T) {
	columns := rawRecordListColumns()
	for _, sensitive := range []string{
		"raw_content",
		"headers_json",
		"query_json",
		"metadata_json",
		"error_message",
		"external_id",
		"dedupe_hash",
	} {
		if slices.Contains(columns, sensitive) {
			t.Fatalf("safe list projection includes %q", sensitive)
		}
	}

	for _, required := range []string{
		"id",
		"source_id",
		"source_code",
		"status",
		"trace_id",
		"received_at",
		"created_at",
	} {
		if !slices.Contains(columns, required) {
			t.Fatalf("safe list projection does not include %q", required)
		}
	}
}
