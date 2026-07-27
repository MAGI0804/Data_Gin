package data_svc

import (
	"encoding/json"
	"testing"
)

func TestFixedMallWeatherExportProfileContainsCompleteWorkbook(t *testing.T) {
	profile, err := fixedMallWeatherExportProfile()
	if err != nil {
		t.Fatalf("fixedMallWeatherExportProfile() error=%v", err)
	}
	var decoded MallWeatherExportProfileConfig
	if err := json.Unmarshal([]byte(profile.ProfileJSON), &decoded); err != nil {
		t.Fatalf("decode fixed profile: %v", err)
	}
	wantKinds := []string{"malls", "realtime", "minutely", "hourly", "daily", "alerts", "life_indices"}
	if profile.Code != fixedMallWeatherExportProfileCode || !profile.Enabled || len(decoded.Datasets) != len(wantKinds) {
		t.Fatalf("profile=%+v config=%+v", profile, decoded)
	}
	if decoded.TimeZone != "Asia/Shanghai" || decoded.UnitSystem != "metric" ||
		decoded.DateFormat != "2006-01-02" || decoded.DateTimeFormat != "2006-01-02 15:04:05" ||
		decoded.FileNameTemplate != "商场天气_{{date:20060102_150405}}.xlsx" {
		t.Fatalf("fixed workbook formats=%+v", decoded)
	}
	for index, want := range wantKinds {
		dataset := decoded.Datasets[index]
		if dataset.Kind != want || !dataset.FreezeHeader || !dataset.AutoFilter {
			t.Fatalf("dataset[%d]=%+v, want kind=%s with fixed presentation", index, dataset, want)
		}
		if want == "malls" {
			if dataset.Latest != nil {
				t.Fatalf("malls dataset unexpectedly has latest=%v", *dataset.Latest)
			}
		} else if dataset.Latest == nil || !*dataset.Latest {
			t.Fatalf("dataset[%d]=%+v, want latest snapshot", index, dataset)
		}
	}
}

func TestFixedMallWeatherExportProfileCodesAreVersionedAndReserved(t *testing.T) {
	if fixedMallWeatherExportProfileCode != fixedMallWeatherExportProfileCodePrefix+"_v1" {
		t.Fatalf("fixed profile code=%q", fixedMallWeatherExportProfileCode)
	}
	for _, code := range []string{
		fixedMallWeatherExportProfileCodePrefix,
		fixedMallWeatherExportProfileCode,
		fixedMallWeatherExportProfileCodePrefix + "_v2",
	} {
		if !reservedFixedMallWeatherExportProfileCode(code) {
			t.Fatalf("reservedFixedMallWeatherExportProfileCode(%q)=false", code)
		}
	}
	if reservedFixedMallWeatherExportProfileCode("mall_weather_excel_custom") {
		t.Fatal("custom profile code was reserved")
	}
}
