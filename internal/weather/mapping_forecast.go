package weather

import (
	"time"

	"gin-biz-web-api/connector/caiyun"
	"gin-biz-web-api/model"
)

func mapRealtime(metadata validatedMappingMetadata, issuedAtUTC time.Time, realtime caiyun.RealtimeWeather, qualityStatus string, qualityFlags model.JSONText) *model.MallWeatherRealtime {
	return &model.MallWeatherRealtime{
		MallID: metadata.MallID, Provider: ProviderCaiyun,
		SnapshotAtUTC: issuedAtUTC, ProviderServerTimeUTC: issuedAtUTC, FetchedAtUTC: metadata.FetchedAtUTC,
		TemperatureC: cloneFloat(realtime.TemperatureC), ApparentTemperatureC: cloneFloat(realtime.ApparentTemperatureC),
		HumidityRatio: cloneFloat(realtime.HumidityRatio), PressurePa: cloneFloat(realtime.PressurePa),
		WindSpeedKPH: cloneFloat(realtime.WindSpeedKPH), WindDirectionDeg: cloneFloat(realtime.WindDirectionDeg),
		CloudrateRatio: cloneFloat(realtime.CloudrateRatio), VisibilityKM: cloneFloat(realtime.VisibilityKM),
		DSWRFWM2: cloneFloat(realtime.DSWRFWM2), Skycon: realtime.Skycon,
		LocalPrecipStatus: realtime.LocalPrecipStatus, LocalPrecipMMH: cloneFloat(realtime.LocalPrecipMMH),
		LocalPrecipDatasource: realtime.LocalPrecipDatasource, NearestPrecipStatus: realtime.NearestPrecipStatus,
		NearestPrecipDistanceKM: cloneFloat(realtime.NearestPrecipDistanceKM), NearestPrecipMMH: cloneFloat(realtime.NearestPrecipMMH),
		PM25UGM3: cloneFloat(realtime.PM25UGM3), PM10UGM3: cloneFloat(realtime.PM10UGM3),
		O3UGM3: cloneFloat(realtime.O3UGM3), SO2UGM3: cloneFloat(realtime.SO2UGM3),
		NO2UGM3: cloneFloat(realtime.NO2UGM3), COMGM3: cloneFloat(realtime.COMGM3),
		AQIChn: cloneInt(realtime.AQIChn), AQIUSA: cloneInt(realtime.AQIUSA),
		AQIDescChn: realtime.AQIDescChn, AQIDescUSA: realtime.AQIDescUSA,
		ComfortIndex: cloneInt(realtime.ComfortIndex), ComfortDesc: realtime.ComfortDesc,
		UltravioletIndex: cloneInt(realtime.UltravioletIndex), UltravioletDesc: realtime.UltravioletDesc,
		WeatherQualityFields: model.WeatherQualityFields{
			FetchRunID: metadata.FetchRunID, QualityStatus: qualityStatus, QualityFlagsJSON: qualityFlags,
			RawChecksum: metadata.RawChecksum, LastSeenAt: metadata.FetchedAtUTC,
		},
	}
}

func mapMinutely(metadata validatedMappingMetadata, bundle *caiyun.MinutelyBundle, metadataWarnings []caiyun.ParseWarning) []model.MallWeatherMinutely {
	warnings := append(append([]caiyun.ParseWarning(nil), metadataWarnings...), bundle.Warnings...)
	qualityFlags, qualityStatus := qualityFields(warnings)
	rows := make([]model.MallWeatherMinutely, len(bundle.Forecasts))
	for index, forecast := range bundle.Forecasts {
		rows[index] = model.MallWeatherMinutely{
			MallID: metadata.MallID, Provider: ProviderCaiyun,
			ForecastMinuteUTC: forecast.ForecastMinuteUTC.UTC(), IssuedAtUTC: bundle.IssuedAtUTC.UTC(), FetchedAtUTC: metadata.FetchedAtUTC,
			MinuteOffset: forecast.MinuteOffset, PrecipitationMMH: cloneFloat(forecast.PrecipitationMMH),
			ProbabilityRatio: cloneFloat(forecast.ProbabilityRatio), ProbabilityWindow: cloneInt(forecast.ProbabilityWindow),
			Datasource: forecast.Datasource, Description: forecast.Description, ForecastKeypoint: forecast.ForecastKeypoint,
			WeatherQualityFields: model.WeatherQualityFields{
				FetchRunID: metadata.FetchRunID, QualityStatus: qualityStatus, QualityFlagsJSON: qualityFlags,
				RawChecksum: metadata.RawChecksum, LastSeenAt: metadata.FetchedAtUTC,
			},
		}
	}
	return rows
}

func mapHourly(metadata validatedMappingMetadata, bundle *caiyun.HourlyBundle, metadataWarnings []caiyun.ParseWarning) []model.MallWeatherHourly {
	warnings := append(append([]caiyun.ParseWarning(nil), metadataWarnings...), bundle.Warnings...)
	qualityFlags, qualityStatus := qualityFields(warnings)
	rows := make([]model.MallWeatherHourly, len(bundle.Forecasts))
	for index, forecast := range bundle.Forecasts {
		rows[index] = model.MallWeatherHourly{
			MallID: metadata.MallID, Provider: ProviderCaiyun,
			ForecastTimeUTC: forecast.ForecastTimeUTC.UTC(), IssuedAtUTC: bundle.IssuedAtUTC.UTC(), FetchedAtUTC: metadata.FetchedAtUTC,
			TemperatureC: cloneFloat(forecast.TemperatureC), ApparentTemperatureC: cloneFloat(forecast.ApparentTemperatureC),
			PressurePa: cloneFloat(forecast.PressurePa), HumidityRatio: cloneFloat(forecast.HumidityRatio),
			WindSpeedKPH: cloneFloat(forecast.WindSpeedKPH), WindDirectionDeg: cloneFloat(forecast.WindDirectionDeg),
			PrecipitationMMH: cloneFloat(forecast.PrecipitationMMH), PrecipProbabilityPct: cloneFloat(forecast.PrecipProbabilityPct),
			CloudrateRatio: cloneFloat(forecast.CloudrateRatio), DSWRFWM2: cloneFloat(forecast.DSWRFWM2),
			VisibilityKM: cloneFloat(forecast.VisibilityKM), Skycon: forecast.Skycon,
			PM25UGM3: cloneFloat(forecast.PM25UGM3), AQIChn: cloneInt(forecast.AQIChn), AQIUSA: cloneInt(forecast.AQIUSA),
			HourlyDescription: forecast.HourlyDescription, ForecastKeypoint: forecast.ForecastKeypoint,
			WeatherQualityFields: model.WeatherQualityFields{
				FetchRunID: metadata.FetchRunID, QualityStatus: qualityStatus, QualityFlagsJSON: qualityFlags,
				RawChecksum: metadata.RawChecksum, LastSeenAt: metadata.FetchedAtUTC,
			},
		}
	}
	return rows
}
