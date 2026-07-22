package data_svc

import (
	"encoding/json"
	"strings"
	"testing"

	"gin-biz-web-api/connector/feishu"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/pkg/credential"
)

func TestMallWeatherFeishuDryRunPlanBlocksUnsafeHeaderWithoutLeakingResources(t *testing.T) {
	t.Parallel()

	input := validMallWeatherFeishuDryRunPlanInput()
	input.Headers["hourly"] = &feishu.SheetValues{Revision: 7, Rows: [][]feishu.SheetCell{{
		{Type: feishu.SheetCellString, Text: "Mall"},
		{Type: feishu.SheetCellString, Text: "Wrong forecast"},
		{Type: feishu.SheetCellString, Text: "Issued"},
	}}}
	result, err := buildMallWeatherFeishuDryRunPlan(input)
	if err != nil {
		t.Fatalf("buildMallWeatherFeishuDryRunPlan() error=%v", err)
	}
	if result.CanExecute || result.TotalEstimatedRows != 10 || result.TotalEstimatedCells != 30 ||
		len(result.Datasets) != 1 {
		t.Fatalf("result=%+v", result)
	}
	plan := result.Datasets[0]
	if plan.HeaderStatus != "MISMATCH" || plan.HeaderAction != "BLOCKED" || plan.CanExecute ||
		plan.RangeStrategy != "UPSERT_MIXED" || plan.PlannedRange != "A:C" ||
		len(plan.HeaderDifferences) != 1 || plan.HeaderDifferences[0].Index != 2 ||
		len(plan.Warnings) != 2 {
		t.Fatalf("plan=%+v", plan)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal() error=%v", err)
	}
	for _, resource := range []string{"spreadsheet-secret-token", "sheet-hourly-secret"} {
		if strings.Contains(string(encoded), resource) {
			t.Fatalf("dry-run result leaks resolved resource %q: %s", resource, encoded)
		}
	}
	if !strings.Contains(string(encoded), credential.EnvFeishuWeatherSpreadsheetToken) ||
		!strings.Contains(string(encoded), credential.EnvFeishuWeatherHourlySheetID) {
		t.Fatalf("dry-run result omits environment references: %s", encoded)
	}
}

func TestMallWeatherFeishuDryRunPlanComputesModeRanges(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		mode           string
		header         *feishu.SheetValues
		estimatedRows  int64
		expectedRange  string
		expectedAction string
	}{
		{
			name: "append to empty sheet", mode: "append", header: &feishu.SheetValues{Rows: [][]feishu.SheetCell{}},
			estimatedRows: 10, expectedRange: "A:C", expectedAction: "WRITE_ON_EXECUTION",
		},
		{
			name: "overwrite matched sheet", mode: "overwrite_range", header: matchedMallWeatherFeishuHeader(),
			estimatedRows: 10, expectedRange: "A2:C11", expectedAction: "NONE",
		},
		{
			name: "overwrite empty dataset", mode: "overwrite_range", header: matchedMallWeatherFeishuHeader(),
			estimatedRows: 0, expectedRange: "", expectedAction: "NONE",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			input := validMallWeatherFeishuDryRunPlanInput()
			input.Destination.Config.WriteMode = test.mode
			input.Destination.Config.UniqueKeyFields = nil
			input.Headers["hourly"] = test.header
			input.EstimatedRows["hourly"] = test.estimatedRows
			result, err := buildMallWeatherFeishuDryRunPlan(input)
			if err != nil {
				t.Fatalf("buildMallWeatherFeishuDryRunPlan() error=%v", err)
			}
			plan := result.Datasets[0]
			if plan.PlannedRange != test.expectedRange || plan.HeaderAction != test.expectedAction || !plan.CanExecute {
				t.Fatalf("plan=%+v", plan)
			}
		})
	}
}

func TestMallWeatherFeishuDryRunPlanRejectsInconsistentInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*mallWeatherFeishuDryRunPlanInput)
	}{
		{
			name: "metadata belongs to another spreadsheet",
			mutate: func(input *mallWeatherFeishuDryRunPlanInput) {
				input.Metadata.SpreadsheetToken = "another-spreadsheet-token"
			},
		},
		{
			name: "split dataset",
			mutate: func(input *mallWeatherFeishuDryRunPlanInput) {
				input.Profile.Datasets[0].SplitBy = "mall"
			},
		},
		{
			name: "unique key is not selected",
			mutate: func(input *mallWeatherFeishuDryRunPlanInput) {
				input.Profile.Datasets[0].Columns = input.Profile.Datasets[0].Columns[:2]
			},
		},
		{
			name: "missing destination mapping",
			mutate: func(input *mallWeatherFeishuDryRunPlanInput) {
				delete(input.Destination.Config.SheetIDEnvMapping, "hourly")
			},
		},
		{
			name: "negative estimate",
			mutate: func(input *mallWeatherFeishuDryRunPlanInput) {
				input.EstimatedRows["hourly"] = -1
			},
		},
		{
			name: "unsafe remote header",
			mutate: func(input *mallWeatherFeishuDryRunPlanInput) {
				input.Headers["hourly"] = &feishu.SheetValues{Rows: [][]feishu.SheetCell{{
					{Type: feishu.SheetCellString, Text: "Mall\nInjected"},
				}}}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			input := validMallWeatherFeishuDryRunPlanInput()
			test.mutate(&input)
			if _, err := buildMallWeatherFeishuDryRunPlan(input); err == nil {
				t.Fatal("buildMallWeatherFeishuDryRunPlan() accepted inconsistent input")
			}
		})
	}
}

func validMallWeatherFeishuDryRunPlanInput() mallWeatherFeishuDryRunPlanInput {
	columns := []requestbody.MallWeatherExportColumn{
		{Field: "mall_code", Title: "Mall"},
		{Field: "forecast_time", Title: "Forecast"},
		{Field: "issued_at", Title: "Issued"},
	}
	return mallWeatherFeishuDryRunPlanInput{
		Destination: &MallWeatherFeishuResolvedDestination{
			DestinationID:    17,
			Code:             "weather_feishu",
			SpreadsheetToken: "spreadsheet-secret-token",
			SheetIDs:         map[string]string{"hourly": "sheet-hourly-secret"},
			Config: MallWeatherFeishuDestinationConfig{
				SpreadsheetTokenEnv: credential.EnvFeishuWeatherSpreadsheetToken,
				SheetIDEnvMapping:   map[string]string{"hourly": credential.EnvFeishuWeatherHourlySheetID},
				WriteMode:           "upsert",
				ProfileCode:         "mall_weather_full",
				UniqueKeyFields:     map[string][]string{"hourly": {"mall_code", "forecast_time", "issued_at"}},
			},
		},
		Profile: MallWeatherExportProfileDTO{
			ID: 9, Code: "mall_weather_full", Version: 3, Enabled: true,
			Datasets: []requestbody.MallWeatherExportDataset{{Kind: "hourly", Columns: columns}},
		},
		Metadata: &feishu.SpreadsheetMetadata{
			SpreadsheetToken: "spreadsheet-secret-token",
			Sheets: []feishu.SheetMetadata{{
				SheetID: "sheet-hourly-secret", Title: "Hourly", Index: 0, ResourceType: "sheet",
				GridProperties: feishu.SheetGridProperties{RowCount: 100, ColumnCount: 10},
			}},
		},
		Headers:       map[string]*feishu.SheetValues{"hourly": matchedMallWeatherFeishuHeader()},
		EstimatedRows: map[string]int64{"hourly": 10},
	}
}

func matchedMallWeatherFeishuHeader() *feishu.SheetValues {
	return &feishu.SheetValues{Revision: 7, Rows: [][]feishu.SheetCell{{
		{Type: feishu.SheetCellString, Text: "Mall"},
		{Type: feishu.SheetCellString, Text: "Forecast"},
		{Type: feishu.SheetCellString, Text: "Issued"},
	}}}
}
