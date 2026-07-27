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
	if _, err := (&MallWeatherExportJobDAO{}).FindByUUIDAndActor(
		context.Background(),
		"00000000-0000-0000-0000-000000000001",
		17,
	); err == nil {
		t.Fatal("FindByUUIDAndActor() accepted an unconfigured DAO")
	}
	if _, err := (&MallWeatherExportJobDAO{}).FindDownloadByUUIDAndActor(
		context.Background(),
		"00000000-0000-0000-0000-000000000001",
		17,
	); err == nil {
		t.Fatal("FindDownloadByUUIDAndActor() accepted an unconfigured DAO")
	}
}

func TestMallWeatherExportJobQueryColumnsExcludeSensitiveData(t *testing.T) {
	selected := make(map[string]bool, len(mallWeatherExportJobQueryColumns))
	for _, column := range mallWeatherExportJobQueryColumns {
		selected[column] = true
	}
	for _, required := range []string{
		"id", "job_uuid", "profile_id", "profile_version", "status", "total_rows", "processed_rows",
		"current_sheet", "cancel_requested", "result_checksum", "file_size_bytes", "error_message_safe",
		"started_at", "finished_at", "expires_at", "created_at", "updated_at",
	} {
		if !selected[required] {
			t.Fatalf("query columns omitted required column %q", required)
		}
	}
	for _, sensitive := range []string{
		"profile_snapshot_json", "filters_json", "idempotency_key", "result_object_key", "last_cursor_json",
	} {
		if selected[sensitive] {
			t.Fatalf("query columns include sensitive column %q", sensitive)
		}
	}
}

func TestMallWeatherExportDownloadQuerySelectsPrivateObjectForActor(t *testing.T) {
	dao := NewMallWeatherExportJobDAO(dryRunWeatherDAOTestDB(t))
	jobUUID := "00000000-0000-4000-8000-000000000017"
	query := dao.downloadByUUIDAndActorQuery(t.Context(), jobUUID, 17).
		Take(&MallWeatherExportDownloadJob{})
	if query.Error != nil {
		t.Fatalf("build download lookup: %v", query.Error)
	}
	statement := query.Statement.SQL.String()
	for _, fragment := range []string{
		"`status`", "`result_object_key`", "`file_size_bytes`", "`expires_at`", "job_uuid = ?", "created_by = ?",
	} {
		if !strings.Contains(statement, fragment) {
			t.Fatalf("download lookup does not contain %q: %s", fragment, statement)
		}
	}
	if strings.Contains(statement, jobUUID) || strings.Contains(statement, "17") {
		t.Fatalf("download lookup interpolated actor-scoped values: %s", statement)
	}
	if len(query.Statement.Vars) != 2 || query.Statement.Vars[0] != jobUUID || query.Statement.Vars[1] != uint(17) {
		t.Fatalf("download lookup vars=%#v", query.Statement.Vars)
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
	var count struct{ Value int64 }
	query = query.Select(countExpression + " AS value").Find(&count)
	if err := query.Error; err != nil {
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

func TestMallWeatherExportEstimateLifeIndicesOnlyUsesComprehensiveSource(t *testing.T) {
	dao := NewMallWeatherExportJobDAO(dryRunWeatherDAOTestDB(t))
	query, countExpression, _, _, _, err := dao.exportEstimateQuery(
		t.Context(), MallWeatherExportEstimateDataset{Kind: "life_indices", Latest: true},
	)
	if err != nil {
		t.Fatalf("exportEstimateQuery() error=%v", err)
	}
	var count struct{ Value int64 }
	query = query.Select(countExpression + " AS value").Find(&count)
	if query.Error != nil {
		t.Fatalf("build estimate query: %v", query.Error)
	}
	if statement := query.Statement.SQL.String(); !strings.Contains(statement, "w.source_api = ?") {
		t.Fatalf("life-index estimate does not restrict comprehensive source: %s", statement)
	}
}

func timePointer(value time.Time) *time.Time {
	return &value
}
