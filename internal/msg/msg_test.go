package msg

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestErrResponseDoesNotExposeTechnicalError(t *testing.T) {
	response := ErrResponse("操作失败，请稍后重试", errors.New("dial tcp db.internal:5432 password=top-secret"))
	payload, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	if strings.Contains(string(payload), "top-secret") || strings.Contains(string(payload), "db.internal") {
		t.Fatalf("response leaked technical error: %s", payload)
	}
	if string(payload) != `{"code":201,"msg":"操作失败，请稍后重试","data":{},"err":""}` {
		t.Fatalf("response = %s", payload)
	}
}
