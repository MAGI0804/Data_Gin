package data_svc

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"gin-biz-web-api/job"
	"gin-biz-web-api/model"
)

func TestNormalizeOfficeMessageInputQuery(t *testing.T) {
	enabled := true
	message, err := normalizeOfficeMessageInput(OfficeMessageInput{
		Name: "销售日报", SourceType: model.OfficeMessageSourceOracleQuery,
		SelectSQL:  "SELECT ORDER_NO, AMOUNT FROM SALES WHERE BILL_DATE = :bill_date",
		Parameters: []OfficeQueryParameter{{Code: "bill_date", Label: "业务日期", ValueType: "date", Format: "yyyyMMdd", Required: true}},
		ColumnMapping: []OfficeColumnMapping{
			{SourceColumn: "ORDER_NO", Header: "单号", ValueType: "string", Order: 1, Width: 20},
			{SourceColumn: "AMOUNT", Header: "金额", ValueType: "decimal", Order: 2, Width: 16},
		}, Enabled: &enabled,
	})
	if err != nil {
		t.Fatalf("normalizeOfficeMessageInput() error = %v", err)
	}
	if message.SourceType != model.OfficeMessageSourceOracleQuery || message.SelectSQL == "" || string(message.ParameterSchemaJSON) == "[]" {
		t.Fatalf("message = %#v", message)
	}
}

func TestNormalizeOfficeMessageInputEditedClearsOracleContract(t *testing.T) {
	message, err := normalizeOfficeMessageInput(OfficeMessageInput{Name: "通知", SourceType: model.OfficeMessageSourceEdited, Content: "系统维护完成"})
	if err != nil {
		t.Fatalf("normalizeOfficeMessageInput() error = %v", err)
	}
	if message.Content != "系统维护完成" || message.SelectSQL != "" || string(message.ColumnMappingJSON) != "[]" {
		t.Fatalf("message = %#v", message)
	}
}

func TestNewOfficePushOutboxContainsOnlyRunID(t *testing.T) {
	runUUID := "9ac63f51-1e15-40b0-ae0a-2b1c29b9de35"
	outbox, err := newOfficePushOutbox(17, runUUID, time.Now().UTC())
	if err != nil {
		t.Fatalf("newOfficePushOutbox() error = %v", err)
	}
	if outbox.TaskType != job.TypeOfficePush || outbox.QueueName != job.OfficePushQueueName || strings.Contains(string(outbox.PayloadJSON), "secret") {
		t.Fatalf("outbox = %#v", outbox)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(outbox.PayloadJSON), &payload); err != nil || len(payload) != 1 || payload["run_id"] != float64(17) {
		t.Fatalf("payload = %#v, %v", payload, err)
	}
}
