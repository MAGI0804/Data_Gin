package weather

import (
	"time"

	"gin-biz-web-api/connector/caiyun"
	"gin-biz-web-api/model"
)

const (
	SourceAPIV26Daily    = "v26_daily"
	SourceAPIV3LifeIndex = "v3_lifeindex"
)

type DailyMappingInput struct {
	Metadata MappingMetadata
	Weather  *caiyun.WeatherBundle
	Daily    *caiyun.DailyBundle
}

type DailyModelBatch struct {
	Daily             []model.MallWeatherDaily
	LifeIndices       []model.MallWeatherLifeIndex
	ParseWarningsJSON model.JSONText
	RowCountsJSON     model.JSONText
}

type LifeIndexMappingInput struct {
	Metadata    MappingMetadata
	IssuedAtUTC time.Time
	LifeIndex   *caiyun.LifeIndexBundle
}

type LifeIndexModelBatch struct {
	LifeIndices       []model.MallWeatherLifeIndex
	ParseWarningsJSON model.JSONText
	RowCountsJSON     model.JSONText
}

func MapDaily(input DailyMappingInput) (*DailyModelBatch, error) {
	metadata, err := validateMappingMetadata(input.Metadata)
	if err != nil || input.Weather == nil || input.Daily == nil || !validMappingTime(input.Daily.IssuedAtUTC) ||
		!input.Daily.IssuedAtUTC.Equal(input.Weather.Metadata.ServerTimeUTC) || !validDailyRawJSON(input.Daily) {
		return nil, ErrInvalidMappingInput
	}
	warnings := append(topLevelWeatherWarnings(input.Weather.Warnings), input.Daily.Warnings...)
	qualityFlags, qualityStatus := qualityFields(warnings)
	dailyRows := make([]model.MallWeatherDaily, len(input.Daily.Forecasts))
	lifeRows := make([]model.MallWeatherLifeIndex, 0)
	for index, forecast := range input.Daily.Forecasts {
		dailyRows[index] = mapDailyRow(metadata, input.Daily.IssuedAtUTC.UTC(), forecast, qualityStatus, qualityFlags)
		lifeRows = append(lifeRows, mapLifeIndexItems(
			metadata, SourceAPIV26Daily, forecast.ForecastDateLocal, input.Daily.IssuedAtUTC.UTC(),
			forecast.BasicLifeIndices, qualityStatus, qualityFlags,
		)...)
	}
	warningsJSON, err := marshalJSONText(warnings)
	if err != nil {
		return nil, ErrInvalidMappingInput
	}
	rowCountsJSON, err := marshalJSONText(map[string]int{"daily": len(dailyRows), "life_index": len(lifeRows)})
	if err != nil {
		return nil, ErrInvalidMappingInput
	}
	return &DailyModelBatch{
		Daily: dailyRows, LifeIndices: lifeRows,
		ParseWarningsJSON: warningsJSON, RowCountsJSON: rowCountsJSON,
	}, nil
}

func MapLifeIndices(input LifeIndexMappingInput) (*LifeIndexModelBatch, error) {
	metadata, err := validateMappingMetadata(input.Metadata)
	if err != nil || input.LifeIndex == nil || !validMappingTime(input.IssuedAtUTC) || !validLifeIndexRawJSON(input.LifeIndex) {
		return nil, ErrInvalidMappingInput
	}
	qualityFlags, qualityStatus := qualityFields(input.LifeIndex.Warnings)
	rows := make([]model.MallWeatherLifeIndex, 0)
	for _, day := range input.LifeIndex.Days {
		rows = append(rows, mapLifeIndexItems(
			metadata, SourceAPIV3LifeIndex, day.Date, input.IssuedAtUTC.UTC(), day.Items, qualityStatus, qualityFlags,
		)...)
	}
	warningsJSON, err := marshalJSONText(input.LifeIndex.Warnings)
	if err != nil {
		return nil, ErrInvalidMappingInput
	}
	rowCountsJSON, err := marshalJSONText(map[string]int{"life_index": len(rows)})
	if err != nil {
		return nil, ErrInvalidMappingInput
	}
	return &LifeIndexModelBatch{LifeIndices: rows, ParseWarningsJSON: warningsJSON, RowCountsJSON: rowCountsJSON}, nil
}

func validDailyRawJSON(bundle *caiyun.DailyBundle) bool {
	for _, forecast := range bundle.Forecasts {
		if !validOptionalJSON(forecast.BasicLifeIndexJSON) ||
			(len(forecast.BasicLifeIndices) > 0 && !validRequiredJSON(forecast.BasicLifeIndexJSON)) {
			return false
		}
		for _, item := range forecast.BasicLifeIndices {
			if item.Code == "" || !validRequiredJSON(item.ProviderJSON) {
				return false
			}
		}
	}
	return true
}

func validLifeIndexRawJSON(bundle *caiyun.LifeIndexBundle) bool {
	for _, day := range bundle.Days {
		if day.Date.IsZero() {
			return false
		}
		for _, item := range day.Items {
			if item.Code == "" || !validRequiredJSON(item.ProviderJSON) {
				return false
			}
		}
	}
	return true
}

func mapDailyRow(metadata validatedMappingMetadata, issuedAtUTC time.Time, forecast caiyun.DailyForecast, qualityStatus string, qualityFlags model.JSONText) model.MallWeatherDaily {
	return model.MallWeatherDaily{
		MallID: metadata.MallID, Provider: ProviderCaiyun,
		ForecastDateLocal: forecast.ForecastDateLocal, IssuedAtUTC: issuedAtUTC, FetchedAtUTC: metadata.FetchedAtUTC,
		TemperatureMaxC: cloneFloat(forecast.Temperature.Maximum), TemperatureMinC: cloneFloat(forecast.Temperature.Minimum), TemperatureAvgC: cloneFloat(forecast.Temperature.Average),
		DayTemperatureMaxC: cloneFloat(forecast.DayTemperature.Maximum), DayTemperatureMinC: cloneFloat(forecast.DayTemperature.Minimum), DayTemperatureAvgC: cloneFloat(forecast.DayTemperature.Average),
		NightTemperatureMaxC: cloneFloat(forecast.NightTemperature.Maximum), NightTemperatureMinC: cloneFloat(forecast.NightTemperature.Minimum), NightTemperatureAvgC: cloneFloat(forecast.NightTemperature.Average),
		PrecipitationMaxMMH: cloneFloat(forecast.Precipitation.Maximum), PrecipitationMinMMH: cloneFloat(forecast.Precipitation.Minimum), PrecipitationAvgMMH: cloneFloat(forecast.Precipitation.Average), PrecipitationProbabilityPct: cloneFloat(forecast.Precipitation.ProbabilityPct),
		DayPrecipitationMaxMMH: cloneFloat(forecast.DayPrecipitation.Maximum), DayPrecipitationMinMMH: cloneFloat(forecast.DayPrecipitation.Minimum), DayPrecipitationAvgMMH: cloneFloat(forecast.DayPrecipitation.Average), DayPrecipitationProbabilityPct: cloneFloat(forecast.DayPrecipitation.ProbabilityPct),
		NightPrecipitationMaxMMH: cloneFloat(forecast.NightPrecipitation.Maximum), NightPrecipitationMinMMH: cloneFloat(forecast.NightPrecipitation.Minimum), NightPrecipitationAvgMMH: cloneFloat(forecast.NightPrecipitation.Average), NightPrecipitationProbabilityPct: cloneFloat(forecast.NightPrecipitation.ProbabilityPct),
		WindMaxSpeedKPH: cloneFloat(forecast.Wind.Maximum.SpeedKPH), WindMaxDirectionDeg: cloneFloat(forecast.Wind.Maximum.DirectionDeg),
		WindMinSpeedKPH: cloneFloat(forecast.Wind.Minimum.SpeedKPH), WindMinDirectionDeg: cloneFloat(forecast.Wind.Minimum.DirectionDeg),
		WindAvgSpeedKPH: cloneFloat(forecast.Wind.Average.SpeedKPH), WindAvgDirectionDeg: cloneFloat(forecast.Wind.Average.DirectionDeg),
		DayWindMaxSpeedKPH: cloneFloat(forecast.DayWind.Maximum.SpeedKPH), DayWindMaxDirectionDeg: cloneFloat(forecast.DayWind.Maximum.DirectionDeg),
		DayWindMinSpeedKPH: cloneFloat(forecast.DayWind.Minimum.SpeedKPH), DayWindMinDirectionDeg: cloneFloat(forecast.DayWind.Minimum.DirectionDeg),
		DayWindAvgSpeedKPH: cloneFloat(forecast.DayWind.Average.SpeedKPH), DayWindAvgDirectionDeg: cloneFloat(forecast.DayWind.Average.DirectionDeg),
		NightWindMaxSpeedKPH: cloneFloat(forecast.NightWind.Maximum.SpeedKPH), NightWindMaxDirectionDeg: cloneFloat(forecast.NightWind.Maximum.DirectionDeg),
		NightWindMinSpeedKPH: cloneFloat(forecast.NightWind.Minimum.SpeedKPH), NightWindMinDirectionDeg: cloneFloat(forecast.NightWind.Minimum.DirectionDeg),
		NightWindAvgSpeedKPH: cloneFloat(forecast.NightWind.Average.SpeedKPH), NightWindAvgDirectionDeg: cloneFloat(forecast.NightWind.Average.DirectionDeg),
		HumidityMaxRatio: cloneFloat(forecast.Humidity.Maximum), HumidityMinRatio: cloneFloat(forecast.Humidity.Minimum), HumidityAvgRatio: cloneFloat(forecast.Humidity.Average),
		CloudrateMaxRatio: cloneFloat(forecast.Cloudrate.Maximum), CloudrateMinRatio: cloneFloat(forecast.Cloudrate.Minimum), CloudrateAvgRatio: cloneFloat(forecast.Cloudrate.Average),
		PressureMaxPa: cloneFloat(forecast.Pressure.Maximum), PressureMinPa: cloneFloat(forecast.Pressure.Minimum), PressureAvgPa: cloneFloat(forecast.Pressure.Average),
		VisibilityMaxKM: cloneFloat(forecast.Visibility.Maximum), VisibilityMinKM: cloneFloat(forecast.Visibility.Minimum), VisibilityAvgKM: cloneFloat(forecast.Visibility.Average),
		DSWRFMaxWM2: cloneFloat(forecast.DSWRF.Maximum), DSWRFMinWM2: cloneFloat(forecast.DSWRF.Minimum), DSWRFAvgWM2: cloneFloat(forecast.DSWRF.Average),
		PM25MaxUGM3: cloneFloat(forecast.AirQuality.PM25.Maximum), PM25MinUGM3: cloneFloat(forecast.AirQuality.PM25.Minimum), PM25AvgUGM3: cloneFloat(forecast.AirQuality.PM25.Average),
		AQIMaxChn: cloneInt(forecast.AirQuality.AQIChn.Maximum), AQIMinChn: cloneInt(forecast.AirQuality.AQIChn.Minimum), AQIAvgChn: cloneInt(forecast.AirQuality.AQIChn.Average),
		AQIMaxUSA: cloneInt(forecast.AirQuality.AQIUSA.Maximum), AQIMinUSA: cloneInt(forecast.AirQuality.AQIUSA.Minimum), AQIAvgUSA: cloneInt(forecast.AirQuality.AQIUSA.Average),
		Skycon: forecast.Skycon, DaySkycon: forecast.DaySkycon, NightSkycon: forecast.NightSkycon,
		SunriseLocalTime: forecast.SunriseLocalTime, SunsetLocalTime: forecast.SunsetLocalTime,
		BasicLifeIndexJSON: model.JSONText(string(forecast.BasicLifeIndexJSON)),
		WeatherQualityFields: model.WeatherQualityFields{
			FetchRunID: metadata.FetchRunID, QualityStatus: qualityStatus, QualityFlagsJSON: qualityFlags,
			RawChecksum: metadata.RawChecksum, LastSeenAt: metadata.FetchedAtUTC,
		},
	}
}

func mapLifeIndexItems(metadata validatedMappingMetadata, sourceAPI string, date, issuedAtUTC time.Time, items []caiyun.LifeIndexItem, qualityStatus string, qualityFlags model.JSONText) []model.MallWeatherLifeIndex {
	rows := make([]model.MallWeatherLifeIndex, len(items))
	for index, item := range items {
		rows[index] = model.MallWeatherLifeIndex{
			MallID: metadata.MallID, Provider: ProviderCaiyun, SourceAPI: sourceAPI,
			ForecastDateLocal: date, IndexType: item.Type, IssuedAtUTC: issuedAtUTC, FetchedAtUTC: metadata.FetchedAtUTC,
			IndexCode: item.Code, IndexName: item.Name, Level: cloneInt(item.Level),
			ShortDesc: item.Description, Detail: item.Detail, IsUnknownType: item.UnknownType,
			ProviderPayloadJSON: model.JSONText(string(item.ProviderJSON)),
			WeatherQualityFields: model.WeatherQualityFields{
				FetchRunID: metadata.FetchRunID, QualityStatus: qualityStatus, QualityFlagsJSON: qualityFlags,
				RawChecksum: metadata.RawChecksum, LastSeenAt: metadata.FetchedAtUTC,
			},
		}
	}
	return rows
}
