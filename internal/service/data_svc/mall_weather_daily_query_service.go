package data_svc

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/model"
)

type MallWeatherDailyDTO struct {
	ForecastDateLocal                string                  `json:"forecastDateLocal"`
	IssuedAtUTC                      time.Time               `json:"issuedAtUtc"`
	IssuedAtLocal                    string                  `json:"issuedAtLocal"`
	FetchedAtUTC                     time.Time               `json:"fetchedAtUtc"`
	FetchedAtLocal                   string                  `json:"fetchedAtLocal"`
	TemperatureMaxC                  *float64                `json:"temperatureMaxC,omitempty"`
	TemperatureMinC                  *float64                `json:"temperatureMinC,omitempty"`
	TemperatureAvgC                  *float64                `json:"temperatureAvgC,omitempty"`
	DayTemperatureMaxC               *float64                `json:"dayTemperatureMaxC,omitempty"`
	DayTemperatureMinC               *float64                `json:"dayTemperatureMinC,omitempty"`
	DayTemperatureAvgC               *float64                `json:"dayTemperatureAvgC,omitempty"`
	NightTemperatureMaxC             *float64                `json:"nightTemperatureMaxC,omitempty"`
	NightTemperatureMinC             *float64                `json:"nightTemperatureMinC,omitempty"`
	NightTemperatureAvgC             *float64                `json:"nightTemperatureAvgC,omitempty"`
	PrecipitationMaxMMH              *float64                `json:"precipitationMaxMmH,omitempty"`
	PrecipitationMinMMH              *float64                `json:"precipitationMinMmH,omitempty"`
	PrecipitationAvgMMH              *float64                `json:"precipitationAvgMmH,omitempty"`
	PrecipitationProbabilityPct      *float64                `json:"precipitationProbabilityPct,omitempty"`
	DayPrecipitationMaxMMH           *float64                `json:"dayPrecipitationMaxMmH,omitempty"`
	DayPrecipitationMinMMH           *float64                `json:"dayPrecipitationMinMmH,omitempty"`
	DayPrecipitationAvgMMH           *float64                `json:"dayPrecipitationAvgMmH,omitempty"`
	DayPrecipitationProbabilityPct   *float64                `json:"dayPrecipitationProbabilityPct,omitempty"`
	NightPrecipitationMaxMMH         *float64                `json:"nightPrecipitationMaxMmH,omitempty"`
	NightPrecipitationMinMMH         *float64                `json:"nightPrecipitationMinMmH,omitempty"`
	NightPrecipitationAvgMMH         *float64                `json:"nightPrecipitationAvgMmH,omitempty"`
	NightPrecipitationProbabilityPct *float64                `json:"nightPrecipitationProbabilityPct,omitempty"`
	WindMaxSpeedKPH                  *float64                `json:"windMaxSpeedKph,omitempty"`
	WindMaxDirectionDeg              *float64                `json:"windMaxDirectionDeg,omitempty"`
	WindMinSpeedKPH                  *float64                `json:"windMinSpeedKph,omitempty"`
	WindMinDirectionDeg              *float64                `json:"windMinDirectionDeg,omitempty"`
	WindAvgSpeedKPH                  *float64                `json:"windAvgSpeedKph,omitempty"`
	WindAvgDirectionDeg              *float64                `json:"windAvgDirectionDeg,omitempty"`
	DayWindMaxSpeedKPH               *float64                `json:"dayWindMaxSpeedKph,omitempty"`
	DayWindMaxDirectionDeg           *float64                `json:"dayWindMaxDirectionDeg,omitempty"`
	DayWindMinSpeedKPH               *float64                `json:"dayWindMinSpeedKph,omitempty"`
	DayWindMinDirectionDeg           *float64                `json:"dayWindMinDirectionDeg,omitempty"`
	DayWindAvgSpeedKPH               *float64                `json:"dayWindAvgSpeedKph,omitempty"`
	DayWindAvgDirectionDeg           *float64                `json:"dayWindAvgDirectionDeg,omitempty"`
	NightWindMaxSpeedKPH             *float64                `json:"nightWindMaxSpeedKph,omitempty"`
	NightWindMaxDirectionDeg         *float64                `json:"nightWindMaxDirectionDeg,omitempty"`
	NightWindMinSpeedKPH             *float64                `json:"nightWindMinSpeedKph,omitempty"`
	NightWindMinDirectionDeg         *float64                `json:"nightWindMinDirectionDeg,omitempty"`
	NightWindAvgSpeedKPH             *float64                `json:"nightWindAvgSpeedKph,omitempty"`
	NightWindAvgDirectionDeg         *float64                `json:"nightWindAvgDirectionDeg,omitempty"`
	HumidityMaxRatio                 *float64                `json:"humidityMaxRatio,omitempty"`
	HumidityMinRatio                 *float64                `json:"humidityMinRatio,omitempty"`
	HumidityAvgRatio                 *float64                `json:"humidityAvgRatio,omitempty"`
	HumidityMaxPct                   *float64                `json:"humidityMaxPct,omitempty"`
	HumidityMinPct                   *float64                `json:"humidityMinPct,omitempty"`
	HumidityAvgPct                   *float64                `json:"humidityAvgPct,omitempty"`
	CloudrateMaxRatio                *float64                `json:"cloudrateMaxRatio,omitempty"`
	CloudrateMinRatio                *float64                `json:"cloudrateMinRatio,omitempty"`
	CloudrateAvgRatio                *float64                `json:"cloudrateAvgRatio,omitempty"`
	PressureMaxPa                    *float64                `json:"pressureMaxPa,omitempty"`
	PressureMinPa                    *float64                `json:"pressureMinPa,omitempty"`
	PressureAvgPa                    *float64                `json:"pressureAvgPa,omitempty"`
	VisibilityMaxKM                  *float64                `json:"visibilityMaxKm,omitempty"`
	VisibilityMinKM                  *float64                `json:"visibilityMinKm,omitempty"`
	VisibilityAvgKM                  *float64                `json:"visibilityAvgKm,omitempty"`
	DSWRFMaxWM2                      *float64                `json:"dswrfMaxWM2,omitempty"`
	DSWRFMinWM2                      *float64                `json:"dswrfMinWM2,omitempty"`
	DSWRFAvgWM2                      *float64                `json:"dswrfAvgWM2,omitempty"`
	PM25MaxUGM3                      *float64                `json:"pm25MaxUgM3,omitempty"`
	PM25MinUGM3                      *float64                `json:"pm25MinUgM3,omitempty"`
	PM25AvgUGM3                      *float64                `json:"pm25AvgUgM3,omitempty"`
	AQIMaxChn                        *int                    `json:"aqiMaxChn,omitempty"`
	AQIMinChn                        *int                    `json:"aqiMinChn,omitempty"`
	AQIAvgChn                        *int                    `json:"aqiAvgChn,omitempty"`
	AQIMaxUSA                        *int                    `json:"aqiMaxUsa,omitempty"`
	AQIMinUSA                        *int                    `json:"aqiMinUsa,omitempty"`
	AQIAvgUSA                        *int                    `json:"aqiAvgUsa,omitempty"`
	Skycon                           string                  `json:"skycon,omitempty"`
	DaySkycon                        string                  `json:"daySkycon,omitempty"`
	NightSkycon                      string                  `json:"nightSkycon,omitempty"`
	SunriseLocalTime                 string                  `json:"sunriseLocalTime,omitempty"`
	SunsetLocalTime                  string                  `json:"sunsetLocalTime,omitempty"`
	QualityStatus                    string                  `json:"qualityStatus"`
	QualityWarnings                  []MallWeatherWarningDTO `json:"qualityWarnings"`
}

type MallWeatherDailyResult struct {
	Items      []MallWeatherDailyDTO `json:"items"`
	Meta       MallWeatherQueryMeta  `json:"meta"`
	Pagination MallWeatherPagination `json:"pagination"`
}

type weatherDailyCursor struct {
	Version           int    `json:"v"`
	ForecastDateLocal string `json:"forecastDateLocal"`
	IssuedAtUnixMS    int64  `json:"issuedAtUnixMs"`
	ID                uint   `json:"id"`
}

func (service *MallWeatherQueryService) Daily(ctx context.Context, actorUserID, mallID uint, request requestbody.MallWeatherDailyQueryRequest) (*MallWeatherDailyResult, error) {
	if service == nil || ctx == nil || mallID == 0 {
		return nil, fmt.Errorf("%w: invalid request", ErrMallWeatherInvalidQuery)
	}
	if err := service.authorize(ctx, actorUserID); err != nil {
		return nil, err
	}
	mall, err := service.malls.FindByID(ctx, mallID)
	if err != nil {
		return nil, err
	}
	location, normalized, err := normalizeDailyWeatherRequest(request, mall)
	if err != nil {
		return nil, err
	}
	cursor, err := decodeWeatherDailyCursor(normalized.Cursor)
	if err != nil {
		return nil, err
	}
	query := data_dao.DailyQuery{
		MallID: mallID, StartLocal: weatherLocalDate(normalized.StartUTC, location), EndLocal: weatherLocalDate(normalized.EndUTC, location),
		AsOfUTC: normalized.AsOfUTC, Latest: normalized.Latest, QualityStatus: normalized.QualityStatus,
		Limit: normalized.PageSize + 1,
	}
	if cursor != nil {
		forecastDate, err := time.Parse("2006-01-02", cursor.ForecastDateLocal)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid cursor", ErrMallWeatherInvalidQuery)
		}
		issuedAt := time.UnixMilli(cursor.IssuedAtUnixMS).UTC()
		query.AfterForecastDateLocal = &forecastDate
		query.AfterIssuedAtUTC = &issuedAt
		query.AfterID = cursor.ID
	}
	rows, err := service.weather.QueryDaily(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("mall weather query: daily rows: %w", err)
	}
	hasMore := len(rows) > normalized.PageSize
	if hasMore {
		rows = rows[:normalized.PageSize]
	}
	items := make([]MallWeatherDailyDTO, len(rows))
	for index := range rows {
		items[index], err = dailyWeatherDTO(&rows[index], location)
		if err != nil {
			return nil, err
		}
	}
	result := &MallWeatherDailyResult{Items: items, Meta: weatherQueryMeta(mall, location, model.MallWeatherFreshnessFresh, nil), Pagination: MallWeatherPagination{PageSize: normalized.PageSize}}
	latest, err := service.weather.FindCurrentLatest(ctx, mallID, model.MallWeatherDataKindDaily)
	if err != nil && !errors.Is(err, data_dao.ErrMallWeatherLatestNotFound) {
		return nil, fmt.Errorf("mall weather query: daily freshness: %w", err)
	}
	if latest == nil {
		result.Meta.FreshnessStatus = "UNAVAILABLE"
	} else {
		status, age, err := currentWeatherFreshness(model.MallWeatherDataKindDaily, latest, service.now().UTC())
		if err != nil {
			return nil, err
		}
		result.Meta.FreshnessStatus = strings.ToUpper(status)
		result.Meta.DataAgeSeconds = &age
	}
	if hasMore && len(rows) > 0 {
		result.Pagination.NextCursor, err = encodeWeatherDailyCursor(&rows[len(rows)-1])
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func normalizeDailyWeatherRequest(request requestbody.MallWeatherDailyQueryRequest, mall *model.Mall) (*time.Location, requestbody.MallWeatherDailyQueryRequest, error) {
	location, normalized, err := normalizeWeatherTimeSeriesRequest(weatherTimeSeriesRequest{
		StartUTC: request.StartUTC, EndUTC: request.EndUTC, TimeZone: request.TimeZone,
		Latest: request.Latest, AsOfUTC: request.AsOfUTC, QualityStatus: request.QualityStatus,
		Cursor: request.Cursor, PageSize: request.PageSize,
	}, mall)
	if err != nil {
		return nil, request, err
	}
	if !weatherLocalDate(normalized.StartUTC, location).Before(weatherLocalDate(normalized.EndUTC, location)) {
		return nil, request, fmt.Errorf("%w: daily range must span at least one local date", ErrMallWeatherInvalidQuery)
	}
	request.StartUTC, request.EndUTC, request.TimeZone = normalized.StartUTC, normalized.EndUTC, normalized.TimeZone
	request.Latest, request.AsOfUTC, request.QualityStatus = normalized.Latest, normalized.AsOfUTC, normalized.QualityStatus
	request.Cursor, request.PageSize = normalized.Cursor, normalized.PageSize
	return location, request, nil
}

func weatherLocalDate(value time.Time, location *time.Location) time.Time {
	local := value.UTC().In(location)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
}

func dailyWeatherDTO(row *model.MallWeatherDaily, location *time.Location) (MallWeatherDailyDTO, error) {
	if row == nil || row.ID == 0 || row.ForecastDateLocal.IsZero() || row.IssuedAtUTC.IsZero() || row.FetchedAtUTC.IsZero() || location == nil {
		return MallWeatherDailyDTO{}, fmt.Errorf("mall weather query: invalid daily row")
	}
	warnings, err := weatherQualityWarnings(row.QualityFlagsJSON)
	if err != nil {
		return MallWeatherDailyDTO{}, err
	}
	return MallWeatherDailyDTO{
		ForecastDateLocal: row.ForecastDateLocal.Format("2006-01-02"),
		IssuedAtUTC:       row.IssuedAtUTC.UTC(), IssuedAtLocal: formatWeatherLocalTime(row.IssuedAtUTC, location),
		FetchedAtUTC: row.FetchedAtUTC.UTC(), FetchedAtLocal: formatWeatherLocalTime(row.FetchedAtUTC, location),
		TemperatureMaxC: row.TemperatureMaxC, TemperatureMinC: row.TemperatureMinC, TemperatureAvgC: row.TemperatureAvgC,
		DayTemperatureMaxC: row.DayTemperatureMaxC, DayTemperatureMinC: row.DayTemperatureMinC, DayTemperatureAvgC: row.DayTemperatureAvgC,
		NightTemperatureMaxC: row.NightTemperatureMaxC, NightTemperatureMinC: row.NightTemperatureMinC, NightTemperatureAvgC: row.NightTemperatureAvgC,
		PrecipitationMaxMMH: row.PrecipitationMaxMMH, PrecipitationMinMMH: row.PrecipitationMinMMH, PrecipitationAvgMMH: row.PrecipitationAvgMMH,
		PrecipitationProbabilityPct: row.PrecipitationProbabilityPct,
		DayPrecipitationMaxMMH:      row.DayPrecipitationMaxMMH, DayPrecipitationMinMMH: row.DayPrecipitationMinMMH, DayPrecipitationAvgMMH: row.DayPrecipitationAvgMMH,
		DayPrecipitationProbabilityPct: row.DayPrecipitationProbabilityPct,
		NightPrecipitationMaxMMH:       row.NightPrecipitationMaxMMH, NightPrecipitationMinMMH: row.NightPrecipitationMinMMH, NightPrecipitationAvgMMH: row.NightPrecipitationAvgMMH,
		NightPrecipitationProbabilityPct: row.NightPrecipitationProbabilityPct,
		WindMaxSpeedKPH:                  row.WindMaxSpeedKPH, WindMaxDirectionDeg: row.WindMaxDirectionDeg,
		WindMinSpeedKPH: row.WindMinSpeedKPH, WindMinDirectionDeg: row.WindMinDirectionDeg,
		WindAvgSpeedKPH: row.WindAvgSpeedKPH, WindAvgDirectionDeg: row.WindAvgDirectionDeg,
		DayWindMaxSpeedKPH: row.DayWindMaxSpeedKPH, DayWindMaxDirectionDeg: row.DayWindMaxDirectionDeg,
		DayWindMinSpeedKPH: row.DayWindMinSpeedKPH, DayWindMinDirectionDeg: row.DayWindMinDirectionDeg,
		DayWindAvgSpeedKPH: row.DayWindAvgSpeedKPH, DayWindAvgDirectionDeg: row.DayWindAvgDirectionDeg,
		NightWindMaxSpeedKPH: row.NightWindMaxSpeedKPH, NightWindMaxDirectionDeg: row.NightWindMaxDirectionDeg,
		NightWindMinSpeedKPH: row.NightWindMinSpeedKPH, NightWindMinDirectionDeg: row.NightWindMinDirectionDeg,
		NightWindAvgSpeedKPH: row.NightWindAvgSpeedKPH, NightWindAvgDirectionDeg: row.NightWindAvgDirectionDeg,
		HumidityMaxRatio: row.HumidityMaxRatio, HumidityMinRatio: row.HumidityMinRatio, HumidityAvgRatio: row.HumidityAvgRatio,
		HumidityMaxPct: ratioPercent(row.HumidityMaxRatio), HumidityMinPct: ratioPercent(row.HumidityMinRatio), HumidityAvgPct: ratioPercent(row.HumidityAvgRatio),
		CloudrateMaxRatio: row.CloudrateMaxRatio, CloudrateMinRatio: row.CloudrateMinRatio, CloudrateAvgRatio: row.CloudrateAvgRatio,
		PressureMaxPa: row.PressureMaxPa, PressureMinPa: row.PressureMinPa, PressureAvgPa: row.PressureAvgPa,
		VisibilityMaxKM: row.VisibilityMaxKM, VisibilityMinKM: row.VisibilityMinKM, VisibilityAvgKM: row.VisibilityAvgKM,
		DSWRFMaxWM2: row.DSWRFMaxWM2, DSWRFMinWM2: row.DSWRFMinWM2, DSWRFAvgWM2: row.DSWRFAvgWM2,
		PM25MaxUGM3: row.PM25MaxUGM3, PM25MinUGM3: row.PM25MinUGM3, PM25AvgUGM3: row.PM25AvgUGM3,
		AQIMaxChn: row.AQIMaxChn, AQIMinChn: row.AQIMinChn, AQIAvgChn: row.AQIAvgChn,
		AQIMaxUSA: row.AQIMaxUSA, AQIMinUSA: row.AQIMinUSA, AQIAvgUSA: row.AQIAvgUSA,
		Skycon: row.Skycon, DaySkycon: row.DaySkycon, NightSkycon: row.NightSkycon,
		SunriseLocalTime: row.SunriseLocalTime, SunsetLocalTime: row.SunsetLocalTime,
		QualityStatus: strings.ToUpper(row.QualityStatus), QualityWarnings: warnings,
	}, nil
}

func encodeWeatherDailyCursor(row *model.MallWeatherDaily) (string, error) {
	if row == nil || row.ID == 0 || row.ForecastDateLocal.IsZero() || row.IssuedAtUTC.IsZero() {
		return "", fmt.Errorf("mall weather query: invalid daily cursor row")
	}
	payload, err := json.Marshal(weatherDailyCursor{Version: 1, ForecastDateLocal: row.ForecastDateLocal.Format("2006-01-02"), IssuedAtUnixMS: row.IssuedAtUTC.UTC().UnixMilli(), ID: row.ID})
	if err != nil {
		return "", fmt.Errorf("mall weather query: encode daily cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func decodeWeatherDailyCursor(value string) (*weatherDailyCursor, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	if len(value) > maxWeatherCursorLength {
		return nil, fmt.Errorf("%w: invalid cursor", ErrMallWeatherInvalidQuery)
	}
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid cursor", ErrMallWeatherInvalidQuery)
	}
	var cursor weatherDailyCursor
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cursor); err != nil || cursor.Version != 1 || cursor.ID == 0 ||
		cursor.IssuedAtUnixMS < minWeatherCursorUnixMS || cursor.IssuedAtUnixMS >= maxWeatherCursorUnixMS {
		return nil, fmt.Errorf("%w: invalid cursor", ErrMallWeatherInvalidQuery)
	}
	date, err := time.Parse("2006-01-02", cursor.ForecastDateLocal)
	if err != nil || date.Year() < 2000 || date.Year() >= 2100 {
		return nil, fmt.Errorf("%w: invalid cursor", ErrMallWeatherInvalidQuery)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%w: invalid cursor", ErrMallWeatherInvalidQuery)
	}
	return &cursor, nil
}
