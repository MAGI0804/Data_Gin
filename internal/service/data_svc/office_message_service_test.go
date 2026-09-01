package data_svc

import (
	"encoding/json"
	"errors"
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

func TestOfficePushSnapshotFreezesReceiverAndMessage(t *testing.T) {
	target := model.OfficePushTarget{ReceiveIDType: "chat_id", ReceiveID: "oc_original"}
	message := model.OfficeMessage{
		BaseModel: model.BaseModel{ID: 7}, Name: "销售日报", SourceType: model.OfficeMessageSourceOracleQuery,
		SelectSQL:           "SELECT ORDER_NO FROM SALES WHERE BILL_DATE = :bill_date",
		ParameterSchemaJSON: model.JSONText(`[{"code":"bill_date","label":"业务日期","valueType":"date","format":"yyyyMMdd","required":true}]`),
		ColumnMappingJSON:   model.JSONText(`[{"sourceColumn":"ORDER_NO","header":"单号","valueType":"string","order":0,"width":18}]`),
	}
	raw, err := newOfficePushSnapshot(target, message)
	if err != nil {
		t.Fatalf("newOfficePushSnapshot() error = %v", err)
	}
	target.ReceiveID, message.SelectSQL = "oc_changed", "SELECT 1 FROM DUAL"
	snapshot, err := decodeOfficePushSnapshot(raw)
	if err != nil {
		t.Fatalf("decodeOfficePushSnapshot() error = %v", err)
	}
	if snapshot.targetModel().ReceiveID != "oc_original" || snapshot.messageModel().SelectSQL == message.SelectSQL {
		t.Fatalf("snapshot = %#v", snapshot)
	}
}

func TestValidateOfficeRunReplayRequiresSameCanonicalParameters(t *testing.T) {
	message := model.OfficeMessage{
		BaseModel: model.BaseModel{ID: 7}, Name: "销售日报", SourceType: model.OfficeMessageSourceOracleQuery,
		SelectSQL:           "SELECT ORDER_NO FROM SALES WHERE BILL_DATE = :bill_date",
		ParameterSchemaJSON: model.JSONText(`[{"code":"bill_date","label":"业务日期","valueType":"date","format":"yyyyMMdd","required":true}]`),
		ColumnMappingJSON:   model.JSONText(`[{"sourceColumn":"ORDER_NO","header":"单号","valueType":"string","order":0,"width":18}]`),
	}
	snapshot, err := newOfficePushSnapshot(model.OfficePushTarget{ReceiveIDType: "chat_id", ReceiveID: "oc_original"}, message)
	if err != nil {
		t.Fatalf("newOfficePushSnapshot() error = %v", err)
	}
	existing := model.OfficePushRun{TargetID: 3, RequestedBy: 5, SnapshotJSON: snapshot, ParametersJSON: model.JSONText(`{ "bill_date" : "20260901" }`)}
	if err := validateOfficeRunReplay(existing, 5, 3, map[string]string{"BILL_DATE": "20260901"}); err != nil {
		t.Fatalf("validateOfficeRunReplay() same request error = %v", err)
	}
	if err := validateOfficeRunReplay(existing, 5, 3, map[string]string{"bill_date": "20260902"}); !errors.Is(err, ErrOfficeMessageConflict) {
		t.Fatalf("validateOfficeRunReplay() mismatched parameters error = %v", err)
	}
}

func TestCanonicalOfficeUUIDNormalizesAcceptedForms(t *testing.T) {
	canonical := "9ac63f51-1e15-40b0-ae0a-2b1c29b9de35"
	got, err := canonicalOfficeUUID("urn:uuid:" + canonical)
	if err != nil || got != canonical {
		t.Fatalf("canonicalOfficeUUID() = %q, %v", got, err)
	}
}
