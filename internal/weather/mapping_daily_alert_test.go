package weather

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"gin-biz-web-api/connector/caiyun"
)

func TestMapDailyMapsMetricsAndBasicLifeIndices(t *testing.T) {
	issuedAt := time.Date(2026, 7, 22, 2, 3, 47, 0, time.UTC)
	value := 34.0
	level := 4
	input := DailyMappingInput{
		Metadata: MappingMetadata{MallID: 3, FetchRunID: 4, FetchedAtUTC: issuedAt.Add(time.Minute), RawChecksum: strings.Repeat("e", 64)},
		Weather:  &caiyun.WeatherBundle{Metadata: caiyun.WeatherMetadata{ServerTimeUTC: issuedAt}},
		Daily: &caiyun.DailyBundle{
			IssuedAtUTC: issuedAt,
			Forecasts: []caiyun.DailyForecast{{
				ForecastDateLocal: time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC),
				Temperature:       caiyun.DailyMetric{Maximum: &value},
				Wind:              caiyun.DailyWind{Maximum: caiyun.DailyWindValue{SpeedKPH: &value}},
				Skycon:            "CLEAR_DAY", BasicLifeIndexJSON: json.RawMessage(`{"ultraviolet":{"index":4}}`),
				BasicLifeIndices: []caiyun.LifeIndexItem{{
					Type: 26, Code: "ULTRAVIOLET", Name: "紫外线/防晒", Level: &level,
					Description: "强", ProviderJSON: json.RawMessage(`{"type":26,"level":4}`),
				}},
			}},
		},
	}
	batch, err := MapDaily(input)
	if err != nil {
		t.Fatalf("MapDaily() error=%v", err)
	}
	if len(batch.Daily) != 1 || len(batch.LifeIndices) != 1 || batch.Daily[0].TemperatureMaxC == nil || *batch.Daily[0].TemperatureMaxC != 34 ||
		batch.Daily[0].WindMaxSpeedKPH == nil || *batch.Daily[0].WindMaxSpeedKPH != 34 || batch.Daily[0].Skycon != "CLEAR_DAY" ||
		batch.LifeIndices[0].SourceAPI != SourceAPIV26Daily || batch.LifeIndices[0].IndexType != 26 || batch.LifeIndices[0].Level == nil || *batch.LifeIndices[0].Level != 4 {
		t.Fatalf("batch=%+v", batch)
	}
	value = -1
	level = -1
	if *batch.Daily[0].TemperatureMaxC != 34 || *batch.LifeIndices[0].Level != 4 {
		t.Fatalf("models alias daily DTO: %+v", batch)
	}
}

func TestMapLifeIndicesMapsV3Rows(t *testing.T) {
	issuedAt := time.Date(2026, 7, 22, 2, 3, 47, 0, time.UTC)
	level := 2
	batch, err := MapLifeIndices(LifeIndexMappingInput{
		Metadata:    MappingMetadata{MallID: 1, FetchRunID: 2, FetchedAtUTC: issuedAt, RawChecksum: strings.Repeat("f", 64)},
		IssuedAtUTC: issuedAt,
		LifeIndex: &caiyun.LifeIndexBundle{Days: []caiyun.LifeIndexDay{{
			Date:  time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC),
			Items: []caiyun.LifeIndexItem{{Type: 1, Code: "AIR_CONDITIONER", Name: "空调", Level: &level, ProviderJSON: json.RawMessage(`{"type":1}`)}},
		}}},
	})
	if err != nil || len(batch.LifeIndices) != 1 || batch.LifeIndices[0].SourceAPI != SourceAPIV3LifeIndex {
		t.Fatalf("batch=%+v error=%v", batch, err)
	}
}

func TestMapAlertsMapsAuditFieldsAndDefensiveCoordinates(t *testing.T) {
	fetchedAt := time.Date(2026, 7, 22, 2, 3, 47, 0, time.UTC)
	latitude := 31.2
	longitude := 121.5
	publishedAt := fetchedAt.Add(-time.Minute)
	batch, err := MapAlerts(AlertMappingInput{
		Metadata: MappingMetadata{MallID: 9, FetchRunID: 10, FetchedAtUTC: fetchedAt, RawChecksum: strings.Repeat("1", 64)},
		Weather:  &caiyun.WeatherBundle{Metadata: caiyun.WeatherMetadata{ServerTimeUTC: fetchedAt.Add(-time.Second)}},
		Alerts: &caiyun.AlertBundle{Alerts: []caiyun.WeatherAlert{{
			AlertID: "alert-1", Status: "预警中", Code: "0902", Latitude: &latitude, Longitude: &longitude,
			PublishedAtUTC: &publishedAt, AdcodesJSON: json.RawMessage(`[{"adcode":"310000"}]`), ProviderJSON: json.RawMessage(`{"alertId":"alert-1"}`),
		}}},
	})
	if err != nil || len(batch.Alerts) != 1 {
		t.Fatalf("batch=%+v error=%v", batch, err)
	}
	alert := batch.Alerts[0]
	if alert.FetchRunID != 10 || alert.FirstSeenAt != fetchedAt || alert.LastSeenAt != fetchedAt || alert.PublishedAtUTC == nil ||
		*alert.Latitude != 31.2 || *alert.Longitude != 121.5 || !json.Valid([]byte(alert.ProviderPayloadJSON)) {
		t.Fatalf("alert=%+v", alert)
	}
	latitude = 0
	longitude = 0
	publishedAt = time.Time{}
	if *alert.Latitude != 31.2 || *alert.Longitude != 121.5 || alert.PublishedAtUTC.IsZero() {
		t.Fatalf("alert aliases DTO: %+v", alert)
	}
}

func TestDailyAndAlertMappersRejectInvalidRawJSON(t *testing.T) {
	issuedAt := time.Date(2026, 7, 22, 2, 3, 47, 0, time.UTC)
	metadata := MappingMetadata{MallID: 1, FetchRunID: 2, FetchedAtUTC: issuedAt, RawChecksum: strings.Repeat("2", 64)}
	weather := &caiyun.WeatherBundle{Metadata: caiyun.WeatherMetadata{ServerTimeUTC: issuedAt}}
	_, dailyErr := MapDaily(DailyMappingInput{
		Metadata: metadata, Weather: weather,
		Daily: &caiyun.DailyBundle{IssuedAtUTC: issuedAt, Forecasts: []caiyun.DailyForecast{{
			ForecastDateLocal: issuedAt, BasicLifeIndices: []caiyun.LifeIndexItem{{Code: "COMFORT", ProviderJSON: []byte(`bad`)}},
		}}},
	})
	_, alertErr := MapAlerts(AlertMappingInput{
		Metadata: metadata, Weather: weather,
		Alerts: &caiyun.AlertBundle{Alerts: []caiyun.WeatherAlert{{AlertID: "a", ProviderJSON: []byte(`bad`)}}},
	})
	if !errors.Is(dailyErr, ErrInvalidMappingInput) || !errors.Is(alertErr, ErrInvalidMappingInput) {
		t.Fatalf("daily error=%v alert error=%v", dailyErr, alertErr)
	}
}
