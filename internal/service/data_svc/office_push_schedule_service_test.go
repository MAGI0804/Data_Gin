package data_svc

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"gin-biz-web-api/model"
)

func TestNextOfficeScheduleTimeUsesFiveFieldsAndShanghai(t *testing.T) {
	after := time.Date(2026, 9, 1, 0, 30, 0, 0, time.UTC)
	next, err := nextOfficeScheduleTime("0 9 * * *", after)
	if err != nil {
		t.Fatalf("nextOfficeScheduleTime() error = %v", err)
	}
	want := time.Date(2026, 9, 1, 1, 0, 0, 0, time.UTC)
	if !next.Equal(want) {
		t.Fatalf("nextOfficeScheduleTime() = %s, want %s", next, want)
	}
	for _, expression := range []string{"", "0 0 9 * * *", "invalid cron"} {
		if _, err := nextOfficeScheduleTime(expression, after); err == nil {
			t.Fatalf("nextOfficeScheduleTime(%q) accepted invalid cron", expression)
		}
	}
}

func TestRenderOfficeScheduleParametersUsesScheduledDateAndOffset(t *testing.T) {
	message := officeScheduleQueryMessage()
	configuration := map[string]OfficeScheduleParameterValue{
		"bill_date": {Mode: model.OfficeScheduleParameterScheduledDate, OffsetDays: -1},
		"store_id":  {Mode: model.OfficeScheduleParameterLiteral, Value: "23"},
	}
	raw, err := normalizeOfficeScheduleParameters(message, configuration)
	if err != nil {
		t.Fatalf("normalizeOfficeScheduleParameters() error = %v", err)
	}
	values, err := renderOfficeScheduleParameters(message, raw, time.Date(2026, 9, 1, 17, 30, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("renderOfficeScheduleParameters() error = %v", err)
	}
	if values["bill_date"] != "20260901" || values["store_id"] != "23" {
		t.Fatalf("renderOfficeScheduleParameters() = %#v", values)
	}
}

func TestNormalizeOfficeScheduleParametersRejectsInvalidModes(t *testing.T) {
	message := officeScheduleQueryMessage()
	tests := []map[string]OfficeScheduleParameterValue{
		{"store_id": {Mode: model.OfficeScheduleParameterLiteral, Value: "23"}},
		{"bill_date": {Mode: model.OfficeScheduleParameterScheduledDate}, "store_id": {Mode: model.OfficeScheduleParameterScheduledDate}},
		{"bill_date": {Mode: model.OfficeScheduleParameterScheduledDate}, "store_id": {Mode: model.OfficeScheduleParameterLiteral, Value: "x"}},
	}
	for _, parameters := range tests {
		if _, err := normalizeOfficeScheduleParameters(message, parameters); !errors.Is(err, ErrOfficeMessageInvalid) && err == nil {
			t.Fatalf("normalizeOfficeScheduleParameters(%#v) accepted invalid values", parameters)
		}
	}
}

func TestOfficeScheduleParametersEncodeWithoutExecutionValues(t *testing.T) {
	raw, err := normalizeOfficeScheduleParameters(officeScheduleQueryMessage(), map[string]OfficeScheduleParameterValue{
		"bill_date": {Mode: model.OfficeScheduleParameterScheduledDate},
		"store_id":  {Mode: model.OfficeScheduleParameterLiteral, Value: "23"},
	})
	if err != nil {
		t.Fatalf("normalizeOfficeScheduleParameters() error = %v", err)
	}
	var values map[string]OfficeScheduleParameterValue
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		t.Fatalf("decode schedule parameters: %v", err)
	}
	if values["bill_date"].Value != "" || values["bill_date"].Mode != model.OfficeScheduleParameterScheduledDate {
		t.Fatalf("schedule parameters = %#v", values)
	}
}

func officeScheduleQueryMessage() model.OfficeMessage {
	return model.OfficeMessage{
		SourceType: model.OfficeMessageSourceOracleQuery,
		SelectSQL:  "SELECT BILLDATE FROM SALES WHERE BILLDATE = :bill_date AND STORE_ID = :store_id",
		ParameterSchemaJSON: model.JSONText(`[
			{"code":"bill_date","label":"业务日期","valueType":"date","format":"yyyyMMdd","required":true},
			{"code":"store_id","label":"门店","valueType":"integer","required":true}
		]`),
	}
}
