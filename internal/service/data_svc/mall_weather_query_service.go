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

	"gin-biz-web-api/connector/caiyun"
	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/internal/requestbody"
	weatherdomain "gin-biz-web-api/internal/weather"
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/database"
)

const (
	defaultWeatherQueryPageSize = 200
	maxWeatherQueryPageSize     = 200
	maxWeatherQueryRange        = 31 * 24 * time.Hour
	maxWeatherCursorLength      = 512
	minWeatherCursorUnixMS      = int64(946684800000)
	maxWeatherCursorUnixMS      = int64(4102444800000)
)

var (
	ErrMallWeatherInvalidQuery          = errors.New("mall weather query: invalid input")
	ErrMallWeatherCoordinateUnconfirmed = errors.New("mall weather query: coordinate unconfirmed")
)

type mallWeatherQueryDAO interface {
	QueryHourly(ctx context.Context, query data_dao.HourlyQuery) ([]model.MallWeatherHourly, error)
	FindCurrentLatest(ctx context.Context, mallID uint, dataKind string) (*model.MallWeatherLatest, error)
}

type mallWeatherQueryMallReader interface {
	FindByID(ctx context.Context, id uint) (*model.Mall, error)
}

type MallWeatherQueryService struct {
	malls       mallWeatherQueryMallReader
	weather     mallWeatherQueryDAO
	permissions mallPermissionChecker
	now         func() time.Time
}

type MallWeatherWarningDTO struct {
	Code string `json:"code"`
	Path string `json:"path"`
}

type MallWeatherHourlyDTO struct {
	ForecastTimeUTC             time.Time               `json:"forecastTimeUtc"`
	ForecastTimeLocal           string                  `json:"forecastTimeLocal"`
	IssuedAtUTC                 time.Time               `json:"issuedAtUtc"`
	IssuedAtLocal               string                  `json:"issuedAtLocal"`
	FetchedAtUTC                time.Time               `json:"fetchedAtUtc"`
	FetchedAtLocal              string                  `json:"fetchedAtLocal"`
	TemperatureC                *float64                `json:"temperatureC,omitempty"`
	ApparentTemperatureC        *float64                `json:"apparentTemperatureC,omitempty"`
	PressurePa                  *float64                `json:"pressurePa,omitempty"`
	HumidityRatio               *float64                `json:"humidityRatio,omitempty"`
	HumidityPct                 *float64                `json:"humidityPct,omitempty"`
	WindSpeedKPH                *float64                `json:"windSpeedKph,omitempty"`
	WindDirectionDeg            *float64                `json:"windDirectionDeg,omitempty"`
	PrecipitationMMH            *float64                `json:"precipitationMmH,omitempty"`
	PrecipitationProbabilityPct *float64                `json:"precipitationProbabilityPct,omitempty"`
	CloudrateRatio              *float64                `json:"cloudrateRatio,omitempty"`
	DSWRFWM2                    *float64                `json:"dswrfWM2,omitempty"`
	VisibilityKM                *float64                `json:"visibilityKm,omitempty"`
	Skycon                      string                  `json:"skycon,omitempty"`
	PM25UGM3                    *float64                `json:"pm25UgM3,omitempty"`
	AQIChn                      *int                    `json:"aqiChn,omitempty"`
	AQIUSA                      *int                    `json:"aqiUsa,omitempty"`
	HourlyDescription           string                  `json:"hourlyDescription,omitempty"`
	ForecastKeypoint            string                  `json:"forecastKeypoint,omitempty"`
	QualityStatus               string                  `json:"qualityStatus"`
	QualityWarnings             []MallWeatherWarningDTO `json:"qualityWarnings"`
}

type MallWeatherQueryMeta struct {
	Provider            string  `json:"provider"`
	APIVersion          string  `json:"apiVersion"`
	RepresentativePoint string  `json:"representativePoint"`
	Longitude           float64 `json:"longitude"`
	Latitude            float64 `json:"latitude"`
	CoordinateSystem    string  `json:"coordinateSystem"`
	SamplingMode        string  `json:"samplingMode"`
	CoverageRadiusM     int     `json:"coverageRadiusM"`
	SpatialResolution   string  `json:"spatialResolution"`
	TimeZone            string  `json:"timeZone"`
	Unit                string  `json:"unit"`
	FreshnessStatus     string  `json:"freshnessStatus"`
	DataAgeSeconds      *int64  `json:"dataAgeSeconds,omitempty"`
}

type MallWeatherPagination struct {
	NextCursor string `json:"nextCursor,omitempty"`
	PageSize   int    `json:"pageSize"`
}

type MallWeatherHourlyResult struct {
	Items      []MallWeatherHourlyDTO `json:"items"`
	Meta       MallWeatherQueryMeta   `json:"meta"`
	Pagination MallWeatherPagination  `json:"pagination"`
}

type weatherHourlyCursor struct {
	Version            int   `json:"v"`
	ForecastTimeUnixMS int64 `json:"forecastTimeUnixMs"`
	IssuedAtUnixMS     int64 `json:"issuedAtUnixMs,omitempty"`
	ID                 uint  `json:"id"`
}

func NewMallWeatherQueryService() *MallWeatherQueryService {
	return &MallWeatherQueryService{
		malls: data_dao.NewMallDAO(database.DB), weather: data_dao.NewMallWeatherDAO(database.DB),
		permissions: data_dao.NewMallWeatherPermissionDAO(database.DB), now: time.Now,
	}
}

func newMallWeatherQueryService(
	malls mallWeatherQueryMallReader,
	weather mallWeatherQueryDAO,
	permissions mallPermissionChecker,
	now func() time.Time,
) (*MallWeatherQueryService, error) {
	if malls == nil || weather == nil || permissions == nil || now == nil {
		return nil, fmt.Errorf("mall weather query: invalid service configuration")
	}
	return &MallWeatherQueryService{malls: malls, weather: weather, permissions: permissions, now: now}, nil
}

func (service *MallWeatherQueryService) Hourly(
	ctx context.Context,
	actorUserID uint,
	mallID uint,
	request requestbody.MallWeatherHourlyQueryRequest,
) (*MallWeatherHourlyResult, error) {
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
	location, normalized, err := normalizeHourlyWeatherRequest(request, mall)
	if err != nil {
		return nil, err
	}
	cursor, err := decodeWeatherHourlyCursor(normalized.Cursor)
	if err != nil {
		return nil, err
	}
	query := data_dao.HourlyQuery{
		MallID: mallID, StartUTC: normalized.StartUTC, EndUTC: normalized.EndUTC,
		AsOfUTC: normalized.AsOfUTC, Latest: normalized.Latest, QualityStatus: normalized.QualityStatus,
		Limit: normalized.PageSize + 1,
	}
	if cursor != nil {
		forecastTime := time.UnixMilli(cursor.ForecastTimeUnixMS).UTC()
		query.AfterForecastTime = &forecastTime
		query.AfterID = cursor.ID
		if cursor.IssuedAtUnixMS != 0 {
			issuedAt := time.UnixMilli(cursor.IssuedAtUnixMS).UTC()
			query.AfterIssuedAtUTC = &issuedAt
		}
	}
	rows, err := service.weather.QueryHourly(ctx, query)
	if err != nil {
		return nil, err
	}
	hasMore := len(rows) > normalized.PageSize
	if hasMore {
		rows = rows[:normalized.PageSize]
	}
	items := make([]MallWeatherHourlyDTO, len(rows))
	for index := range rows {
		items[index], err = hourlyWeatherDTO(&rows[index], location)
		if err != nil {
			return nil, err
		}
	}
	result := &MallWeatherHourlyResult{
		Items:      items,
		Meta:       weatherQueryMeta(mall, location, model.MallWeatherFreshnessFresh, nil),
		Pagination: MallWeatherPagination{PageSize: normalized.PageSize},
	}
	latest, err := service.weather.FindCurrentLatest(ctx, mallID, model.MallWeatherDataKindHourly)
	if err != nil && !errors.Is(err, data_dao.ErrMallWeatherLatestNotFound) {
		return nil, err
	}
	if latest == nil {
		result.Meta.FreshnessStatus = "UNAVAILABLE"
	} else {
		status, age, err := currentWeatherFreshness(model.MallWeatherDataKindHourly, latest, service.now().UTC())
		if err != nil {
			return nil, err
		}
		result.Meta.FreshnessStatus = strings.ToUpper(status)
		result.Meta.DataAgeSeconds = &age
	}
	if hasMore && len(rows) > 0 {
		result.Pagination.NextCursor, err = encodeWeatherHourlyCursor(&rows[len(rows)-1])
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (service *MallWeatherQueryService) authorize(ctx context.Context, actorUserID uint) error {
	if actorUserID == 0 {
		return ErrMallForbidden
	}
	allowed, err := service.permissions.HasPermission(ctx, actorUserID, PermissionWeatherRead, service.now().UTC())
	if err != nil {
		return fmt.Errorf("mall weather query: authorize: %w", err)
	}
	if !allowed {
		return ErrMallForbidden
	}
	return nil
}

func normalizeHourlyWeatherRequest(request requestbody.MallWeatherHourlyQueryRequest, mall *model.Mall) (*time.Location, requestbody.MallWeatherHourlyQueryRequest, error) {
	if mall == nil || mall.ID == 0 || mall.WeatherLongitude == nil || mall.WeatherLatitude == nil || mall.GeocodeStatus != "confirmed" ||
		*mall.WeatherLongitude < -180 || *mall.WeatherLongitude > 180 || *mall.WeatherLatitude < -90 || *mall.WeatherLatitude > 90 {
		return nil, request, ErrMallWeatherCoordinateUnconfirmed
	}
	if request.StartUTC.IsZero() || request.EndUTC.IsZero() || !request.StartUTC.Before(request.EndUTC) ||
		request.EndUTC.Sub(request.StartUTC) > maxWeatherQueryRange {
		return nil, request, fmt.Errorf("%w: invalid time range", ErrMallWeatherInvalidQuery)
	}
	request.StartUTC = request.StartUTC.UTC()
	request.EndUTC = request.EndUTC.UTC()
	if request.AsOfUTC != nil {
		asOf := request.AsOfUTC.UTC()
		request.AsOfUTC = &asOf
	}
	request.QualityStatus = strings.ToLower(strings.TrimSpace(request.QualityStatus))
	if request.QualityStatus != "" && request.QualityStatus != weatherdomain.QualityStatusValid && request.QualityStatus != weatherdomain.QualityStatusWarning {
		return nil, request, fmt.Errorf("%w: invalid quality status", ErrMallWeatherInvalidQuery)
	}
	if request.PageSize == 0 {
		request.PageSize = defaultWeatherQueryPageSize
	}
	if request.PageSize < 1 || request.PageSize > maxWeatherQueryPageSize || len(request.Cursor) > maxWeatherCursorLength {
		return nil, request, fmt.Errorf("%w: invalid pagination", ErrMallWeatherInvalidQuery)
	}
	zoneName := strings.TrimSpace(request.TimeZone)
	if zoneName == "" {
		zoneName = strings.TrimSpace(mall.Timezone)
	}
	if zoneName == "" {
		zoneName = "Asia/Shanghai"
	}
	location, err := time.LoadLocation(zoneName)
	if err != nil {
		return nil, request, fmt.Errorf("%w: invalid time zone", ErrMallWeatherInvalidQuery)
	}
	request.TimeZone = zoneName
	return location, request, nil
}

func hourlyWeatherDTO(row *model.MallWeatherHourly, location *time.Location) (MallWeatherHourlyDTO, error) {
	if row == nil || row.ID == 0 || row.ForecastTimeUTC.IsZero() || row.IssuedAtUTC.IsZero() || row.FetchedAtUTC.IsZero() || location == nil {
		return MallWeatherHourlyDTO{}, fmt.Errorf("mall weather query: invalid hourly row")
	}
	warnings := make([]MallWeatherWarningDTO, 0)
	if row.QualityFlagsJSON != "" {
		var parsed []caiyun.ParseWarning
		if err := json.Unmarshal([]byte(row.QualityFlagsJSON), &parsed); err != nil {
			return MallWeatherHourlyDTO{}, fmt.Errorf("mall weather query: decode quality warnings: %w", err)
		}
		warnings = make([]MallWeatherWarningDTO, len(parsed))
		for index := range parsed {
			warnings[index] = MallWeatherWarningDTO{Code: parsed[index].Code, Path: parsed[index].Path}
		}
	}
	return MallWeatherHourlyDTO{
		ForecastTimeUTC: row.ForecastTimeUTC.UTC(), ForecastTimeLocal: formatWeatherLocalTime(row.ForecastTimeUTC, location),
		IssuedAtUTC: row.IssuedAtUTC.UTC(), IssuedAtLocal: formatWeatherLocalTime(row.IssuedAtUTC, location),
		FetchedAtUTC: row.FetchedAtUTC.UTC(), FetchedAtLocal: formatWeatherLocalTime(row.FetchedAtUTC, location),
		TemperatureC: row.TemperatureC, ApparentTemperatureC: row.ApparentTemperatureC,
		PressurePa: row.PressurePa, HumidityRatio: row.HumidityRatio, HumidityPct: ratioPercent(row.HumidityRatio),
		WindSpeedKPH: row.WindSpeedKPH, WindDirectionDeg: row.WindDirectionDeg,
		PrecipitationMMH: row.PrecipitationMMH, PrecipitationProbabilityPct: row.PrecipProbabilityPct,
		CloudrateRatio: row.CloudrateRatio, DSWRFWM2: row.DSWRFWM2, VisibilityKM: row.VisibilityKM,
		Skycon: row.Skycon, PM25UGM3: row.PM25UGM3, AQIChn: row.AQIChn, AQIUSA: row.AQIUSA,
		HourlyDescription: row.HourlyDescription, ForecastKeypoint: row.ForecastKeypoint,
		QualityStatus: strings.ToUpper(row.QualityStatus), QualityWarnings: warnings,
	}, nil
}

func weatherQueryMeta(mall *model.Mall, location *time.Location, freshness string, age *int64) MallWeatherQueryMeta {
	return MallWeatherQueryMeta{
		Provider: "CAIYUN", APIVersion: "v2.6", RepresentativePoint: "MALL_CENTER",
		Longitude: *mall.WeatherLongitude, Latitude: *mall.WeatherLatitude,
		CoordinateSystem: strings.ToUpper(mall.WeatherCoordinateSystem), SamplingMode: strings.ToUpper(mall.SamplingMode),
		CoverageRadiusM:   mall.CoverageRadiusM,
		SpatialResolution: "9-13km; precipitation within first 2h may be 1km",
		TimeZone:          location.String(), Unit: "metric:v2", FreshnessStatus: strings.ToUpper(freshness), DataAgeSeconds: age,
	}
}

func currentWeatherFreshness(dataKind string, latest *model.MallWeatherLatest, now time.Time) (string, int64, error) {
	if latest == nil || latest.FetchedAtUTC.IsZero() || now.IsZero() {
		return "", 0, fmt.Errorf("mall weather query: invalid freshness pointer")
	}
	status := latest.FreshnessStatus
	if status != model.MallWeatherFreshnessStale {
		var err error
		status, err = weatherdomain.FreshnessStatus(dataKind, latest.FetchedAtUTC, now)
		if err != nil {
			return "", 0, err
		}
	}
	age := now.UTC().Sub(latest.FetchedAtUTC.UTC())
	if age < 0 {
		age = 0
	}
	return status, int64(age / time.Second), nil
}

func encodeWeatherHourlyCursor(row *model.MallWeatherHourly) (string, error) {
	if row == nil || row.ID == 0 || row.ForecastTimeUTC.IsZero() || row.IssuedAtUTC.IsZero() {
		return "", fmt.Errorf("mall weather query: invalid cursor row")
	}
	payload, err := json.Marshal(weatherHourlyCursor{
		Version: 1, ForecastTimeUnixMS: row.ForecastTimeUTC.UTC().UnixMilli(),
		IssuedAtUnixMS: row.IssuedAtUTC.UTC().UnixMilli(), ID: row.ID,
	})
	if err != nil {
		return "", fmt.Errorf("mall weather query: encode cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func decodeWeatherHourlyCursor(value string) (*weatherHourlyCursor, error) {
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
	var cursor weatherHourlyCursor
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cursor); err != nil || cursor.Version != 1 || cursor.ID == 0 ||
		cursor.ForecastTimeUnixMS < minWeatherCursorUnixMS || cursor.ForecastTimeUnixMS >= maxWeatherCursorUnixMS ||
		cursor.IssuedAtUnixMS < minWeatherCursorUnixMS || cursor.IssuedAtUnixMS >= maxWeatherCursorUnixMS {
		return nil, fmt.Errorf("%w: invalid cursor", ErrMallWeatherInvalidQuery)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%w: invalid cursor", ErrMallWeatherInvalidQuery)
	}
	return &cursor, nil
}

func formatWeatherLocalTime(value time.Time, location *time.Location) string {
	return value.UTC().In(location).Format(time.RFC3339Nano)
}

func ratioPercent(value *float64) *float64 {
	if value == nil {
		return nil
	}
	percent := *value * 100
	return &percent
}
