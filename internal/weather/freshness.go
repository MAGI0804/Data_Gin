package weather

import (
	"fmt"
	"time"

	"gin-biz-web-api/model"
)

type FreshnessThresholds struct {
	Warning  time.Duration
	Critical time.Duration
}

func FreshnessThresholdsForKind(dataKind string) (FreshnessThresholds, error) {
	var thresholds FreshnessThresholds
	switch dataKind {
	case model.MallWeatherDataKindRealtime, model.MallWeatherDataKindMinutely:
		thresholds = FreshnessThresholds{Warning: 15 * time.Minute, Critical: 30 * time.Minute}
	case model.MallWeatherDataKindHourly:
		thresholds = FreshnessThresholds{Warning: 2 * time.Hour, Critical: 4 * time.Hour}
	case model.MallWeatherDataKindDaily:
		thresholds = FreshnessThresholds{Warning: 12 * time.Hour, Critical: 24 * time.Hour}
	case model.MallWeatherDataKindLife:
		thresholds = FreshnessThresholds{Warning: 3 * time.Hour, Critical: 8 * time.Hour}
	default:
		return FreshnessThresholds{}, fmt.Errorf("mall weather: unsupported freshness data kind")
	}
	return thresholds, nil
}

func FreshnessStatus(dataKind string, fetchedAt, now time.Time) (string, error) {
	if fetchedAt.IsZero() || now.IsZero() {
		return "", fmt.Errorf("mall weather: invalid freshness time")
	}
	thresholds, err := FreshnessThresholdsForKind(dataKind)
	if err != nil {
		return "", err
	}
	age := now.UTC().Sub(fetchedAt.UTC())
	switch {
	case age >= thresholds.Critical:
		return model.MallWeatherFreshnessCritical, nil
	case age >= thresholds.Warning:
		return model.MallWeatherFreshnessWarning, nil
	default:
		return model.MallWeatherFreshnessFresh, nil
	}
}
