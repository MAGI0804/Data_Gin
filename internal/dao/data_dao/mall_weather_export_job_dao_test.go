package data_dao

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestMallWeatherExportJobDAORejectsInvalidStateBeforeDatabase(t *testing.T) {
	_, err := (&MallWeatherExportJobDAO{}).EstimateRows(
		context.Background(),
		MallWeatherExportEstimateRequest{Datasets: []MallWeatherExportEstimateDataset{{Kind: "hourly"}}, StopAfter: 10},
	)
	if err == nil {
		t.Fatal("EstimateRows() accepted an unconfigured DAO")
	}
}

func TestMallWeatherExportEstimateUsesBoundFiltersAndLatestIdentity(t *testing.T) {
	dao := NewMallWeatherExportJobDAO(dryRunWeatherDAOTestDB(t))
	asOf := time.Date(2026, 7, 22, 8, 0, 0, 0, time.UTC)
	dataset := MallWeatherExportEstimateDataset{Kind: "hourly", Latest: true, AsOfUTC: &asOf}
	query, countExpression, timeColumn, qualityColumn, issuedColumn, err := dao.exportEstimateQuery(
		context.Background(),
		dataset,
	)
	if err != nil {
		t.Fatalf("exportEstimateQuery() error=%v", err)
	}
	filter := MallWeatherExportEstimateFilter{
		MallIDs: []uint{7}, Cities: []string{"shanghai"}, MallStatuses: []string{"active"},
		QualityStatuses: []string{"valid"}, StartUTC: &asOf, EndUTC: timePointer(asOf.Add(time.Hour)),
	}
	query = applyMallWeatherExportMallFilters(query, filter).
		Where(qualityColumn+" IN ?", filter.QualityStatuses).
		Where(timeColumn+" >= ?", filter.StartUTC).
		Where(timeColumn+" < ?", filter.EndUTC).
		Where(issuedColumn+" <= ?", dataset.AsOfUTC)
	var count int64
	if err := query.Select(countExpression).Scan(&count).Error; err != nil {
		t.Fatalf("build estimate query: %v", err)
	}
	statement := query.Statement.SQL.String()
	for _, fragment := range []string{
		"COUNT(DISTINCT w.mall_id, w.forecast_time_utc)", "m.id IN", "m.city IN", "m.status IN",
		"w.quality_status IN", "w.forecast_time_utc >=", "w.forecast_time_utc <", "w.issued_at_utc <=",
	} {
		if !strings.Contains(statement, fragment) {
			t.Fatalf("statement does not contain %q: %s", fragment, statement)
		}
	}
	if strings.Contains(statement, "shanghai") || strings.Contains(statement, "active") {
		t.Fatalf("statement interpolated filter values: %s", statement)
	}
}

func timePointer(value time.Time) *time.Time {
	return &value
}
