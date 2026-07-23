package data_svc

import (
	"strings"
	"testing"

	"gin-biz-web-api/connector/feishu"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/pkg/credential"
)

func TestBuildMallWeatherFeishuUpsertRowsUsesTypedDeterministicKeys(t *testing.T) {
	columns := testMallWeatherFeishuUpsertColumns()
	batch := mallWeatherFeishuRenderedBatch{Rows: [][]feishu.SheetCell{
		testMallWeatherFeishuUpsertCells("M001", "2026-07-23T10:00:00Z", "sunny"),
		testMallWeatherFeishuUpsertCells("M002", "2026-07-23T10:00:00Z", "cloudy"),
	}}
	first, err := buildMallWeatherFeishuUpsertRows(columns, []string{"mall_code", "forecast_time"}, batch)
	if err != nil {
		t.Fatalf("buildMallWeatherFeishuUpsertRows() error=%v", err)
	}
	second, err := buildMallWeatherFeishuUpsertRows(columns, []string{"mall_code", "forecast_time"}, batch)
	if err != nil {
		t.Fatalf("buildMallWeatherFeishuUpsertRows() second error=%v", err)
	}
	if len(first) != 2 || first[0].BusinessKey != second[0].BusinessKey ||
		first[0].Checksum != second[0].Checksum || !strings.HasPrefix(first[0].BusinessKey, "sha256:") ||
		len(first[0].BusinessKey) != 71 || len(first[0].Checksum) != 64 ||
		first[0].BusinessKey == first[1].BusinessKey {
		t.Fatalf("unexpected rows=%+v", first)
	}

	changed := batch
	changed.Rows = [][]feishu.SheetCell{
		testMallWeatherFeishuUpsertCells("M001", "2026-07-23T10:00:00Z", "rain"),
	}
	changedRows, err := buildMallWeatherFeishuUpsertRows(
		columns,
		[]string{"mall_code", "forecast_time"},
		changed,
	)
	if err != nil {
		t.Fatalf("buildMallWeatherFeishuUpsertRows() changed error=%v", err)
	}
	if changedRows[0].BusinessKey != first[0].BusinessKey || changedRows[0].Checksum == first[0].Checksum {
		t.Fatalf("business key or checksum did not preserve upsert semantics")
	}
}

func TestBuildMallWeatherFeishuUpsertRowsRejectsAmbiguousKeys(t *testing.T) {
	columns := testMallWeatherFeishuUpsertColumns()
	tests := []struct {
		name string
		rows [][]feishu.SheetCell
		keys []string
	}{
		{
			name: "duplicate key",
			rows: [][]feishu.SheetCell{
				testMallWeatherFeishuUpsertCells("M001", "2026-07-23T10:00:00Z", "sunny"),
				testMallWeatherFeishuUpsertCells("M001", "2026-07-23T10:00:00Z", "rain"),
			},
			keys: []string{"mall_code", "forecast_time"},
		},
		{
			name: "blank key",
			rows: [][]feishu.SheetCell{testMallWeatherFeishuUpsertCells("", "2026-07-23T10:00:00Z", "sunny")},
			keys: []string{"mall_code", "forecast_time"},
		},
		{
			name: "unknown key column",
			rows: [][]feishu.SheetCell{testMallWeatherFeishuUpsertCells("M001", "2026-07-23T10:00:00Z", "sunny")},
			keys: []string{"unknown"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := buildMallWeatherFeishuUpsertRows(
				columns,
				test.keys,
				mallWeatherFeishuRenderedBatch{Rows: test.rows},
			); err == nil {
				t.Fatal("buildMallWeatherFeishuUpsertRows() accepted ambiguous key")
			}
		})
	}
}

func TestMallWeatherFeishuMappingSchemaChecksumTracksResolvedSheet(t *testing.T) {
	destination := testMallWeatherFeishuUpsertDestination()
	columns := testMallWeatherFeishuUpsertColumns()
	first, err := mallWeatherFeishuMappingSchemaChecksum(destination, "hourly", columns)
	if err != nil {
		t.Fatalf("mallWeatherFeishuMappingSchemaChecksum() error=%v", err)
	}
	second, err := mallWeatherFeishuMappingSchemaChecksum(destination, "hourly", columns)
	if err != nil || first != second || len(first) != 64 || strings.Contains(first, "sheet-secret") {
		t.Fatalf("checksum first=%q second=%q error=%v", first, second, err)
	}
	changed := *destination
	changed.SheetIDs = map[string]string{"hourly": "new-sheet-secret"}
	third, err := mallWeatherFeishuMappingSchemaChecksum(&changed, "hourly", columns)
	if err != nil || third == first {
		t.Fatalf("changed checksum=%q original=%q error=%v", third, first, err)
	}
}

func testMallWeatherFeishuUpsertColumns() []requestbody.MallWeatherExportColumn {
	return []requestbody.MallWeatherExportColumn{
		{Field: "mall_code", Title: "Mall"},
		{Field: "forecast_time", Title: "Forecast"},
		{Field: "skycon", Title: "Weather"},
	}
}

func testMallWeatherFeishuUpsertCells(mallCode, forecastTime, skycon string) []feishu.SheetCell {
	return []feishu.SheetCell{
		{Type: feishu.SheetCellString, Text: mallCode},
		{Type: feishu.SheetCellString, Text: forecastTime},
		{Type: feishu.SheetCellString, Text: skycon},
	}
}

func testMallWeatherFeishuUpsertDestination() *MallWeatherFeishuResolvedDestination {
	return &MallWeatherFeishuResolvedDestination{
		DestinationID: 17,
		Code:          "weather_feishu",
		Config: MallWeatherFeishuDestinationConfig{
			SpreadsheetTokenEnv: credential.EnvFeishuWeatherSpreadsheetToken,
			SheetIDEnvMapping:   map[string]string{"hourly": credential.EnvFeishuWeatherHourlySheetID},
			WriteMode:           "upsert",
			BatchRows:           200,
			ProfileCode:         "mall_weather_full",
			UniqueKeyFields:     map[string][]string{"hourly": {"mall_code", "forecast_time"}},
			TimeoutSeconds:      20,
		},
		SpreadsheetToken: "spreadsheet-secret-token",
		SheetIDs:         map[string]string{"hourly": "sheet-secret"},
	}
}
