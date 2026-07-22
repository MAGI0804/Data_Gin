package data_dao

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestAdvanceWeatherObservedAtBuildsMonotonicParameterizedUpdate(t *testing.T) {
	tests := []struct {
		name   string
		column string
	}{
		{name: "success", column: "last_weather_success_at"},
		{name: "error", column: "last_weather_error_at"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			observedAt := time.Date(2026, 7, 22, 8, 30, 0, 0, time.FixedZone("CST", 8*60*60))
			expression, err := monotonicWeatherObservedAtExpr(test.column, observedAt)
			if err != nil {
				t.Fatalf("monotonicWeatherObservedAtExpr() error=%v", err)
			}
			sqlText := expression.SQL
			if strings.Contains(sqlText, observedAt.Format(time.RFC3339)) {
				t.Fatalf("timestamp was interpolated into SQL: %s", sqlText)
			}
			if !strings.Contains(sqlText, "`"+test.column+"`") || !strings.Contains(sqlText, "CASE WHEN") || len(expression.Vars) != 2 {
				t.Fatalf("SQL=%s", sqlText)
			}
			for _, value := range expression.Vars {
				got, ok := value.(time.Time)
				if !ok || got.Location() != time.UTC || !got.Equal(observedAt) {
					t.Fatalf("bound timestamp=%v", value)
				}
			}
		})
	}
}

func TestAdvanceWeatherObservedAtRejectsInvalidInput(t *testing.T) {
	dao := &MallDAO{}
	if err := dao.advanceWeatherObservedAt(context.Background(), 0, time.Now(), true); err == nil {
		t.Fatal("advanceWeatherObservedAt() accepted zero mall id")
	}
	if err := dao.advanceWeatherObservedAt(context.Background(), 1, time.Time{}, true); err == nil {
		t.Fatal("advanceWeatherObservedAt() accepted zero time")
	}
	if _, err := monotonicWeatherObservedAtExpr("status", time.Now()); err == nil {
		t.Fatal("monotonicWeatherObservedAtExpr() accepted an unsafe column")
	}
}
