package weather

import (
	"testing"
	"time"

	"gin-biz-web-api/model"
)

func TestFreshnessStatusUsesDataKindThresholds(t *testing.T) {
	now := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name     string
		dataKind string
		age      time.Duration
		want     string
	}{
		{name: "future realtime is fresh", dataKind: model.MallWeatherDataKindRealtime, age: -time.Minute, want: model.MallWeatherFreshnessFresh},
		{name: "realtime warning boundary", dataKind: model.MallWeatherDataKindRealtime, age: 15 * time.Minute, want: model.MallWeatherFreshnessWarning},
		{name: "minutely critical boundary", dataKind: model.MallWeatherDataKindMinutely, age: 30 * time.Minute, want: model.MallWeatherFreshnessCritical},
		{name: "hourly remains fresh", dataKind: model.MallWeatherDataKindHourly, age: 2*time.Hour - time.Millisecond, want: model.MallWeatherFreshnessFresh},
		{name: "hourly is critical", dataKind: model.MallWeatherDataKindHourly, age: 4 * time.Hour, want: model.MallWeatherFreshnessCritical},
		{name: "daily is warning", dataKind: model.MallWeatherDataKindDaily, age: 12 * time.Hour, want: model.MallWeatherFreshnessWarning},
		{name: "life is critical", dataKind: model.MallWeatherDataKindLife, age: 8 * time.Hour, want: model.MallWeatherFreshnessCritical},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := FreshnessStatus(test.dataKind, now.Add(-test.age), now)
			if err != nil {
				t.Fatalf("FreshnessStatus() error=%v", err)
			}
			if got != test.want {
				t.Fatalf("FreshnessStatus()=%q want=%q", got, test.want)
			}
		})
	}
}

func TestFreshnessStatusRejectsInvalidInput(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name      string
		dataKind  string
		fetchedAt time.Time
		now       time.Time
	}{
		{name: "unknown kind", dataKind: "alerts", fetchedAt: now, now: now},
		{name: "missing fetched time", dataKind: model.MallWeatherDataKindRealtime, now: now},
		{name: "missing current time", dataKind: model.MallWeatherDataKindRealtime, fetchedAt: now},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := FreshnessStatus(test.dataKind, test.fetchedAt, test.now); err == nil {
				t.Fatal("FreshnessStatus() accepted invalid input")
			}
		})
	}
}
