package data_svc

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/model"
)

// MallWeatherCurrentRealtimeResult is the current realtime weather exposed to open API clients.
type MallWeatherCurrentRealtimeResult struct {
	Realtime *MallWeatherRealtimeDTO `json:"realtime"`
	Meta     MallWeatherQueryMeta    `json:"meta"`
}

// CurrentRealtime returns the latest realtime snapshot for one mall.
func (service *MallWeatherQueryService) CurrentRealtime(
	ctx context.Context,
	actorUserID uint,
	mallID uint,
	timeZone string,
) (*MallWeatherCurrentRealtimeResult, error) {
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
	location, err := weatherMallLocation(mall, timeZone)
	if err != nil {
		return nil, err
	}

	result := &MallWeatherCurrentRealtimeResult{
		Meta: weatherQueryMeta(mall, location, "unavailable", nil),
	}
	current, err := service.weather.FindCurrentRealtime(ctx, mallID)
	if errors.Is(err, data_dao.ErrMallWeatherLatestNotFound) {
		return result, nil
	}
	if err != nil {
		return nil, fmt.Errorf("mall weather query: current realtime: %w", err)
	}
	dto, err := realtimeWeatherDTO(&current.Weather, location)
	if err != nil {
		return nil, err
	}
	status, age, err := currentWeatherFreshness(
		model.MallWeatherDataKindRealtime,
		&current.Latest,
		service.now().UTC(),
	)
	if err != nil {
		return nil, err
	}
	result.Realtime = &dto
	result.Meta.FreshnessStatus = strings.ToUpper(status)
	result.Meta.DataAgeSeconds = &age
	return result, nil
}
