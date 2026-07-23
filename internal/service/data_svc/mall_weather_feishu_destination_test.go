package data_svc

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/credential"
)

func TestMallWeatherFeishuDestinationParsesValidUpsertConfig(t *testing.T) {
	t.Parallel()

	config, err := parseMallWeatherFeishuDestinationConfig(validMallWeatherFeishuDestinationJSON(t))
	if err != nil {
		t.Fatalf("parseMallWeatherFeishuDestinationConfig() error=%v", err)
	}
	if config.BatchRows != defaultMallWeatherFeishuBatchRows ||
		config.TimeoutSeconds != defaultMallWeatherFeishuTimeout ||
		config.WriteMode != "upsert" || config.ProfileCode != "mall_weather_full" {
		t.Fatalf("config=%+v", config)
	}
	if fields := config.UniqueKeyFields["hourly"]; len(fields) != 3 || fields[1] != "forecast_time" {
		t.Fatalf("hourly unique fields=%v", fields)
	}
}

func TestMallWeatherFeishuDestinationRejectsInvalidConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{
			name: "unknown field",
			mutate: func(config map[string]any) {
				config["appSecret"] = "must-not-be-accepted"
			},
		},
		{
			name: "arbitrary spreadsheet environment",
			mutate: func(config map[string]any) {
				config["spreadsheetTokenEnv"] = "DB_PASSWORD"
			},
		},
		{
			name: "arbitrary sheet environment",
			mutate: func(config map[string]any) {
				config["sheetIdEnvMapping"].(map[string]string)["hourly"] = "UNRELATED_SECRET"
			},
		},
		{
			name: "unknown dataset",
			mutate: func(config map[string]any) {
				config["sheetIdEnvMapping"] = map[string]string{"fetch_runs": credential.EnvFeishuWeatherHourlySheetID}
				config["unique_key_fields"] = map[string][]string{"fetch_runs": {"mall_code"}}
			},
		},
		{
			name: "normalized duplicate dataset",
			mutate: func(config map[string]any) {
				config["sheetIdEnvMapping"] = map[string]string{
					"hourly":   credential.EnvFeishuWeatherHourlySheetID,
					" HOURLY ": credential.EnvFeishuWeatherDailySheetID,
				}
				config["unique_key_fields"] = map[string][]string{"hourly": {"mall_code"}}
			},
		},
		{
			name: "duplicate sheet reference",
			mutate: func(config map[string]any) {
				config["sheetIdEnvMapping"] = map[string]string{
					"hourly": credential.EnvFeishuWeatherHourlySheetID,
					"daily":  credential.EnvFeishuWeatherHourlySheetID,
				}
				config["unique_key_fields"] = map[string][]string{
					"hourly": {"mall_code"},
					"daily":  {"mall_code"},
				}
			},
		},
		{
			name: "incomplete upsert keys",
			mutate: func(config map[string]any) {
				delete(config["unique_key_fields"].(map[string][]string), "alerts")
			},
		},
		{
			name: "unknown upsert field",
			mutate: func(config map[string]any) {
				config["unique_key_fields"].(map[string][]string)["hourly"] = []string{"mall_code", "password"}
			},
		},
		{
			name: "duplicate upsert field",
			mutate: func(config map[string]any) {
				config["unique_key_fields"].(map[string][]string)["hourly"] = []string{"mall_code", "mall_code"}
			},
		},
		{
			name: "append with unique keys",
			mutate: func(config map[string]any) {
				config["write_mode"] = "append"
			},
		},
		{
			name: "overwrite with unique keys",
			mutate: func(config map[string]any) {
				config["write_mode"] = "overwrite_range"
			},
		},
		{
			name: "batch exceeds limit",
			mutate: func(config map[string]any) {
				config["batch_rows"] = maxMallWeatherFeishuBatchRows + 1
			},
		},
		{
			name: "timeout exceeds limit",
			mutate: func(config map[string]any) {
				config["timeout_seconds"] = maxMallWeatherFeishuTimeout + 1
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			config := validMallWeatherFeishuDestinationMap()
			test.mutate(config)
			raw, err := json.Marshal(config)
			if err != nil {
				t.Fatalf("json.Marshal() error=%v", err)
			}
			if _, err := parseMallWeatherFeishuDestinationConfig(string(raw)); err == nil {
				t.Fatalf("parseMallWeatherFeishuDestinationConfig() accepted %s", raw)
			}
		})
	}
}

func TestMallWeatherFeishuDestinationResolvesResourcesWithoutSnapshotLeak(t *testing.T) {
	t.Parallel()

	const (
		spreadsheetToken = "spreadsheet-secret-token"
		hourlySheetID    = "hourly-secret-sheet"
		alertSheetID     = "alert-secret-sheet"
		lifeSheetID      = "life-secret-sheet"
	)
	resolver := fakeMallWeatherFeishuResourceResolver{values: map[string]string{
		credential.EnvFeishuWeatherSpreadsheetToken: spreadsheetToken,
		credential.EnvFeishuWeatherHourlySheetID:    hourlySheetID,
		credential.EnvFeishuWeatherAlertSheetID:     alertSheetID,
		credential.EnvFeishuWeatherLifeIndexSheetID: lifeSheetID,
	}}
	destination := &model.DestinationDefinition{
		BaseModel:       model.BaseModel{ID: 17},
		Code:            " weather_feishu ",
		DestinationType: mallWeatherFeishuDestinationType,
		ConfigJSON:      validMallWeatherFeishuDestinationJSON(t),
		Enabled:         true,
	}
	resolved, err := resolveMallWeatherFeishuDestination(destination, resolver)
	if err != nil {
		t.Fatalf("resolveMallWeatherFeishuDestination() error=%v", err)
	}
	if resolved.DestinationID != 17 || resolved.Code != "weather_feishu" ||
		resolved.SpreadsheetToken != spreadsheetToken || resolved.SheetIDs["hourly"] != hourlySheetID {
		t.Fatalf("resolved destination metadata is incorrect")
	}
	snapshot, err := mallWeatherFeishuDestinationSnapshot(resolved)
	if err != nil {
		t.Fatalf("mallWeatherFeishuDestinationSnapshot() error=%v", err)
	}
	for _, secret := range []string{spreadsheetToken, hourlySheetID, alertSheetID, lifeSheetID} {
		if strings.Contains(string(snapshot), secret) {
			t.Fatalf("snapshot contains resolved resource %q", secret)
		}
	}
	if !strings.Contains(string(snapshot), credential.EnvFeishuWeatherSpreadsheetToken) ||
		!strings.Contains(string(snapshot), credential.EnvFeishuWeatherHourlySheetID) ||
		!strings.Contains(string(snapshot), `"destinationId":17`) ||
		!strings.Contains(string(snapshot), `"code":"weather_feishu"`) {
		t.Fatalf("snapshot=%s", snapshot)
	}
	serialized, err := json.Marshal(resolved)
	if err != nil {
		t.Fatalf("json.Marshal() error=%v", err)
	}
	if strings.Contains(string(serialized), spreadsheetToken) || strings.Contains(string(serialized), hourlySheetID) {
		t.Fatalf("resolved destination JSON leaks resources: %s", serialized)
	}
	for _, diagnostic := range []string{fmt.Sprintf("%+v", resolved), fmt.Sprintf("%#v", resolved)} {
		if strings.Contains(diagnostic, spreadsheetToken) || strings.Contains(diagnostic, hourlySheetID) {
			t.Fatalf("resolved destination diagnostic formatting leaks resources")
		}
	}
}

func TestMallWeatherFeishuDestinationRejectsDuplicateResolvedSheetIDs(t *testing.T) {
	t.Parallel()

	resolver := fakeMallWeatherFeishuResourceResolver{values: map[string]string{
		credential.EnvFeishuWeatherSpreadsheetToken: "spreadsheet-token",
		credential.EnvFeishuWeatherHourlySheetID:    "same-sheet-id",
		credential.EnvFeishuWeatherAlertSheetID:     "same-sheet-id",
		credential.EnvFeishuWeatherLifeIndexSheetID: "life-sheet-id",
	}}
	destination := &model.DestinationDefinition{
		BaseModel:       model.BaseModel{ID: 17},
		Code:            "weather_feishu",
		DestinationType: mallWeatherFeishuDestinationType,
		ConfigJSON:      validMallWeatherFeishuDestinationJSON(t),
		Enabled:         true,
	}
	if _, err := resolveMallWeatherFeishuDestination(destination, resolver); err == nil {
		t.Fatal("resolveMallWeatherFeishuDestination() accepted duplicate resolved sheet IDs")
	}
}

func TestMallWeatherFeishuDestinationResourceErrorsDoNotLeakValues(t *testing.T) {
	t.Parallel()

	const sensitiveValue = "must-not-appear-in-errors"
	destination := &model.DestinationDefinition{
		BaseModel:       model.BaseModel{ID: 17},
		Code:            "weather_feishu",
		DestinationType: mallWeatherFeishuDestinationType,
		ConfigJSON:      validMallWeatherFeishuDestinationJSON(t),
		Enabled:         true,
	}
	resolver := fakeMallWeatherFeishuResourceResolver{err: errors.New(sensitiveValue)}
	_, err := resolveMallWeatherFeishuDestination(destination, resolver)
	if err == nil || strings.Contains(err.Error(), sensitiveValue) {
		t.Fatalf("resolveMallWeatherFeishuDestination() error=%v", err)
	}
}

type fakeMallWeatherFeishuResourceResolver struct {
	values map[string]string
	err    error
}

func (r fakeMallWeatherFeishuResourceResolver) EnvironmentValue(name string) (string, error) {
	if r.err != nil {
		return "", r.err
	}
	value, ok := r.values[name]
	if !ok {
		return "", errors.New("resource is not configured")
	}
	return value, nil
}

func validMallWeatherFeishuDestinationJSON(t *testing.T) string {
	t.Helper()

	raw, err := json.Marshal(validMallWeatherFeishuDestinationMap())
	if err != nil {
		t.Fatalf("json.Marshal() error=%v", err)
	}
	return string(raw)
}

func validMallWeatherFeishuDestinationMap() map[string]any {
	return map[string]any{
		"spreadsheetTokenEnv": credential.EnvFeishuWeatherSpreadsheetToken,
		"sheetIdEnvMapping": map[string]string{
			"hourly":       credential.EnvFeishuWeatherHourlySheetID,
			"alerts":       credential.EnvFeishuWeatherAlertSheetID,
			"life_indices": credential.EnvFeishuWeatherLifeIndexSheetID,
		},
		"write_mode":   "upsert",
		"profile_code": "mall_weather_full",
		"unique_key_fields": map[string][]string{
			"hourly":       {"mall_code", "forecast_time", "issued_at"},
			"alerts":       {"mall_code", "alert_id"},
			"life_indices": {"mall_code", "forecast_date", "index_type", "issued_at"},
		},
	}
}
