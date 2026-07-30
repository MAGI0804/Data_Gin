package data_svc

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/internal/requestbody"
	weatherdomain "gin-biz-web-api/internal/weather"
)

type OpenWeatherHistoryDaySummaryResult struct {
	ObservationStatus string                           `json:"observationStatus"`
	Summary           *OpenWeatherHistoryDaySummaryDTO `json:"summary"`
	Meta              MallWeatherQueryMeta             `json:"meta"`
}

type OpenWeatherHistoryDaySummaryDTO struct {
	Date                    string    `json:"date"`
	SampleCount             int64     `json:"sampleCount"`
	ObservedStartUTC        time.Time `json:"observedStartUtc"`
	ObservedEndUTC          time.Time `json:"observedEndUtc"`
	ObservedStartLocal      string    `json:"observedStartLocal"`
	ObservedEndLocal        string    `json:"observedEndLocal"`
	DominantSkycon          *string   `json:"dominantSkycon"`
	TemperatureMinC         *float64  `json:"temperatureMinC"`
	TemperatureMaxC         *float64  `json:"temperatureMaxC"`
	TemperatureAvgC         *float64  `json:"temperatureAvgC"`
	ApparentTemperatureAvgC *float64  `json:"apparentTemperatureAvgC"`
	HumidityAvgPct          *float64  `json:"humidityAvgPct"`
	PressureAvgPa           *float64  `json:"pressureAvgPa"`
	WindSpeedAvgKPH         *float64  `json:"windSpeedAvgKph"`
	WindSpeedMaxKPH         *float64  `json:"windSpeedMaxKph"`
	PrecipitationAvgMMH     *float64  `json:"precipitationAvgMmH"`
	PrecipitationMaxMMH     *float64  `json:"precipitationMaxMmH"`
	RainySampleCount        int64     `json:"rainySampleCount"`
	VisibilityMinKM         *float64  `json:"visibilityMinKm"`
	VisibilityAvgKM         *float64  `json:"visibilityAvgKm"`
	PM25AvgUGM3             *float64  `json:"pm25AvgUgM3"`
	PM25MaxUGM3             *float64  `json:"pm25MaxUgM3"`
	AQIChnAvg               *float64  `json:"aqiChnAvg"`
	AQIChnMax               *int      `json:"aqiChnMax"`
}

func (service *MallWeatherQueryService) HistoryDaySummary(
	ctx context.Context,
	actorUserID uint,
	mallID uint,
	request requestbody.OpenWeatherHistoryDaySummaryRequest,
) (*OpenWeatherHistoryDaySummaryResult, error) {
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
	location, err := weatherMallLocation(mall, request.TimeZone)
	if err != nil {
		return nil, err
	}
	date, dateValue, err := service.parseCompletedLocalDay(request.Date, location)
	if err != nil {
		return nil, err
	}
	qualityStatus := strings.ToLower(strings.TrimSpace(request.QualityStatus))
	if qualityStatus != "" && qualityStatus != weatherdomain.QualityStatusValid &&
		qualityStatus != weatherdomain.QualityStatusWarning {
		return nil, fmt.Errorf("%w: invalid quality status", ErrMallWeatherInvalidQuery)
	}
	result := &OpenWeatherHistoryDaySummaryResult{
		ObservationStatus: "UNAVAILABLE",
		Meta:              weatherQueryMeta(mall, location, "unavailable", nil),
	}
	summary, err := service.weather.SummarizeRealtimeDay(ctx, data_dao.RealtimeDaySummaryQuery{
		MallID:   mallID,
		StartUTC: date.UTC(), EndUTC: date.AddDate(0, 0, 1).UTC(),
		QualityStatus: qualityStatus,
	})
	if err != nil {
		return nil, fmt.Errorf("mall weather query: summarize historical day: %w", err)
	}
	if err := service.applyRealtimeFreshness(ctx, mallID, &result.Meta); err != nil {
		return nil, err
	}
	if summary == nil {
		return result, nil
	}
	if summary.ObservedStartUTC == nil || summary.ObservedEndUTC == nil {
		return nil, fmt.Errorf("mall weather query: invalid historical day summary")
	}
	result.Summary = openWeatherHistoryDaySummaryDTO(dateValue, summary, location)
	result.ObservationStatus = "AVAILABLE"
	return result, nil
}

func (service *MallWeatherQueryService) parseCompletedLocalDay(
	value string,
	location *time.Location,
) (time.Time, string, error) {
	dateValue := strings.TrimSpace(value)
	date, err := time.ParseInLocation(openWeatherDateLayout, dateValue, location)
	if err != nil || date.Format(openWeatherDateLayout) != dateValue {
		return time.Time{}, "", fmt.Errorf("%w: invalid date", ErrMallWeatherInvalidQuery)
	}
	today := service.now().In(location)
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, location)
	if !date.Before(today) {
		return time.Time{}, "", fmt.Errorf("%w: date must be in the past", ErrMallWeatherInvalidQuery)
	}
	return date, dateValue, nil
}

func openWeatherHistoryDaySummaryDTO(
	date string,
	summary *data_dao.RealtimeDaySummary,
	location *time.Location,
) *OpenWeatherHistoryDaySummaryDTO {
	return &OpenWeatherHistoryDaySummaryDTO{
		Date: date, SampleCount: summary.SampleCount,
		ObservedStartUTC: summary.ObservedStartUTC.UTC(), ObservedEndUTC: summary.ObservedEndUTC.UTC(),
		ObservedStartLocal: formatWeatherLocalTime(*summary.ObservedStartUTC, location),
		ObservedEndLocal:   formatWeatherLocalTime(*summary.ObservedEndUTC, location),
		DominantSkycon:     summary.DominantSkycon,
		TemperatureMinC:    summary.TemperatureMinC, TemperatureMaxC: summary.TemperatureMaxC,
		TemperatureAvgC: summary.TemperatureAvgC, ApparentTemperatureAvgC: summary.ApparentTemperatureAvgC,
		HumidityAvgPct: ratioPercent(summary.HumidityAvgRatio), PressureAvgPa: summary.PressureAvgPa,
		WindSpeedAvgKPH: summary.WindSpeedAvgKPH, WindSpeedMaxKPH: summary.WindSpeedMaxKPH,
		PrecipitationAvgMMH: summary.PrecipitationAvgMMH, PrecipitationMaxMMH: summary.PrecipitationMaxMMH,
		RainySampleCount: summary.RainySampleCount,
		VisibilityMinKM:  summary.VisibilityMinKM, VisibilityAvgKM: summary.VisibilityAvgKM,
		PM25AvgUGM3: summary.PM25AvgUGM3, PM25MaxUGM3: summary.PM25MaxUGM3,
		AQIChnAvg: summary.AQIChnAvg, AQIChnMax: summary.AQIChnMax,
	}
}
