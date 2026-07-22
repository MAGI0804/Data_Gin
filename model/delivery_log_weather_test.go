package model

import (
	"reflect"
	"sync"
	"testing"

	"gorm.io/gorm/schema"
)

func TestDeliveryLogExposesWeatherBatchCheckpointColumns(t *testing.T) {
	parsed, err := schema.Parse(&DeliveryLog{}, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		t.Fatalf("schema.Parse() error=%v", err)
	}
	want := map[string]string{
		"DatasetKind":     "dataset_kind",
		"BatchNo":         "batch_no",
		"RowStart":        "row_start",
		"RowEnd":          "row_end",
		"RecordCount":     "record_count",
		"CellCount":       "cell_count",
		"RequestChecksum": "request_checksum",
		"Status":          "status",
		"FeishuCode":      "feishu_code",
		"Attempt":         "attempt",
		"ResponseSummary": "response_summary",
		"StartedAt":       "started_at",
		"FinishedAt":      "finished_at",
	}
	got := make(map[string]string, len(want))
	for fieldName := range want {
		field := parsed.LookUpField(fieldName)
		if field == nil {
			t.Fatalf("DeliveryLog field %q is missing", fieldName)
		}
		got[fieldName] = field.DBName
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("weather batch columns=%v want=%v", got, want)
	}
}
