package data_svc

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"gin-biz-web-api/model"

	"github.com/google/uuid"
)

func TestMallWeatherFeishuPushCreateCommandValidation(t *testing.T) {
	command := validMallWeatherFeishuPushCreateCommandForTest()
	if !validMallWeatherFeishuPushCreateCommand(command) {
		t.Fatal("validMallWeatherFeishuPushCreateCommand() rejected valid command")
	}
	tests := []struct {
		name   string
		mutate func(*mallWeatherFeishuPushCreateCommand)
	}{
		{name: "missing actor", mutate: func(command *mallWeatherFeishuPushCreateCommand) { command.ActorUserID = 0 }},
		{name: "invalid destination config", mutate: func(command *mallWeatherFeishuPushCreateCommand) { command.DestinationConfigJSON = `[]` }},
		{name: "invalid profile snapshot", mutate: func(command *mallWeatherFeishuPushCreateCommand) { command.ProfileSnapshotJSON = `{` }},
		{name: "oversized filters", mutate: func(command *mallWeatherFeishuPushCreateCommand) {
			command.FiltersJSON = model.JSONText(`{"city":"` + strings.Repeat("x", 64*1024) + `"}`)
		}},
		{name: "invalid key hash", mutate: func(command *mallWeatherFeishuPushCreateCommand) { command.KeyHash = "short" }},
		{name: "invalid trace", mutate: func(command *mallWeatherFeishuPushCreateCommand) { command.TraceID = "not-a-uuid" }},
		{name: "negative estimate", mutate: func(command *mallWeatherFeishuPushCreateCommand) { command.EstimatedRows = -1 }},
		{name: "missing request time", mutate: func(command *mallWeatherFeishuPushCreateCommand) { command.RequestedAt = time.Time{} }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			invalid := command
			test.mutate(&invalid)
			if validMallWeatherFeishuPushCreateCommand(invalid) {
				t.Fatal("validMallWeatherFeishuPushCreateCommand() accepted invalid command")
			}
		})
	}
}

func TestMallWeatherFeishuPushIdempotencyResultIsStrict(t *testing.T) {
	now := time.Date(2026, 7, 23, 8, 0, 0, 0, time.UTC)
	original := MallWeatherFeishuPushCreateResult{
		RunID: 17, TraceID: uuid.NewString(), Status: "PENDING", DestinationID: 8,
		ProfileID: 9, ProfileVersion: 3, EstimatedRows: 100, CreatedBy: 11, CreatedAt: now,
	}
	encoded, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal() error=%v", err)
	}
	decoded, err := decodeMallWeatherFeishuPushCreateResult(model.JSONText(encoded))
	if err != nil || decoded.RunID != original.RunID || decoded.TraceID != original.TraceID ||
		!decoded.CreatedAt.Equal(now) {
		t.Fatalf("decodeMallWeatherFeishuPushCreateResult() result=%+v error=%v", decoded, err)
	}
	for _, invalid := range []model.JSONText{
		model.JSONText(strings.TrimSuffix(string(encoded), "}") + `,"secret":"x"}`),
		model.JSONText(string(encoded) + `{}`),
		model.JSONText(`{"runId":0}`),
	} {
		if _, err := decodeMallWeatherFeishuPushCreateResult(invalid); err == nil {
			t.Fatalf("decodeMallWeatherFeishuPushCreateResult(%s) accepted invalid response", invalid)
		}
	}
}

func TestMallWeatherFeishuPushStoreRejectsInvalidConfiguration(t *testing.T) {
	store := gormMallWeatherFeishuPushStore{}
	if _, _, err := store.Create(t.Context(), validMallWeatherFeishuPushCreateCommandForTest()); err == nil {
		t.Fatal("Create() accepted an unconfigured store")
	}
}

func validMallWeatherFeishuPushCreateCommandForTest() mallWeatherFeishuPushCreateCommand {
	return mallWeatherFeishuPushCreateCommand{
		ActorUserID:             11,
		DestinationID:           8,
		DestinationCode:         "weather_feishu",
		DestinationConfigJSON:   `{"spreadsheetTokenEnv":"FEISHU_WEATHER_SPREADSHEET_TOKEN"}`,
		ProfileID:               9,
		ProfileVersion:          3,
		ProfileCode:             "mall_weather_full",
		ProfileName:             "Mall weather",
		ProfileJSON:             model.JSONText(`{"timezone":"Asia/Shanghai"}`),
		ProfileSnapshotJSON:     model.JSONText(`{"profileId":9}`),
		FiltersJSON:             model.JSONText(`{"cities":["Shanghai"]}`),
		DestinationSnapshotJSON: model.JSONText(`{"writeMode":"append"}`),
		KeyHash:                 strings.Repeat("a", 64),
		RequestHash:             strings.Repeat("b", 64),
		TraceID:                 uuid.NewString(),
		EstimatedRows:           100,
		RequestedAt:             time.Date(2026, 7, 23, 8, 0, 0, 0, time.UTC),
	}
}
