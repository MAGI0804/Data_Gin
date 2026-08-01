package data_ctrl

import (
	"encoding/json"
	"testing"

	"gin-biz-web-api/internal/service/data_svc"
)

func TestSafeTransformRawRecordResultExcludesCleanContent(t *testing.T) {
	result := safeTransformRawRecordResult(&data_svc.TransformRawRecordResult{
		TraceID:       "trace-safe",
		CleanRecordID: 19,
		CleanContent: map[string]interface{}{
			"token": "must-not-be-returned",
		},
	})

	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	if string(encoded) != `{"clean_record_id":19,"trace_id":"trace-safe"}` {
		t.Fatalf("response result=%s", encoded)
	}
}
