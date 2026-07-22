package data_dao

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"gin-biz-web-api/model"
)

func TestNormalizePageSizes(t *testing.T) {
	tests := []struct {
		name   string
		input  int
		mall   int
		outbox int
	}{
		{"default", 0, 50, 100},
		{"requested", 25, 25, 25},
		{"bounded", 1000, maxMallPageSize, maxOutboxClaimSize},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizePageSize(tt.input); got != tt.mall {
				t.Fatalf("normalizePageSize(%d) = %d, want %d", tt.input, got, tt.mall)
			}
			if got := normalizeOutboxClaimSize(tt.input); got != tt.outbox {
				t.Fatalf("normalizeOutboxClaimSize(%d) = %d, want %d", tt.input, got, tt.outbox)
			}
		})
	}
}

func TestSanitizeMallUpdates(t *testing.T) {
	tests := []struct {
		name      string
		updates   map[string]interface{}
		wantError bool
	}{
		{"allows profile field", map[string]interface{}{"name_cn": "updated"}, false},
		{"rejects immutable code", map[string]interface{}{"mall_code": "changed"}, true},
		{"rejects arbitrary sql field", map[string]interface{}{"name_cn = ?; DROP TABLE malls": "x"}, true},
		{"rejects empty update", map[string]interface{}{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sanitizeMallUpdates(tt.updates)
			if (err != nil) != tt.wantError {
				t.Fatalf("sanitizeMallUpdates() error = %v, wantError %t", err, tt.wantError)
			}
			if err == nil && !reflect.DeepEqual(got, tt.updates) {
				t.Fatalf("sanitizeMallUpdates() = %v, want %v", got, tt.updates)
			}
		})
	}
}

func TestSanitizeFetchRunUpdates(t *testing.T) {
	if _, err := sanitizeFetchRunUpdates(map[string]interface{}{"status": "running"}); err != nil {
		t.Fatalf("sanitizeFetchRunUpdates() rejected status: %v", err)
	}
	if _, err := sanitizeFetchRunUpdates(map[string]interface{}{"task_window": "changed"}); err == nil {
		t.Fatal("sanitizeFetchRunUpdates() accepted immutable task window")
	}
}

func TestSanitizeFetchAttemptUpdates(t *testing.T) {
	if _, err := sanitizeFetchAttemptUpdates(map[string]interface{}{"status": "success"}); err != nil {
		t.Fatalf("sanitizeFetchAttemptUpdates() rejected status: %v", err)
	}
	if _, err := sanitizeFetchAttemptUpdates(map[string]interface{}{"attempt_no": 99}); err == nil {
		t.Fatal("sanitizeFetchAttemptUpdates() accepted immutable attempt number")
	}
}

func TestClassifyFetchAttemptStart(t *testing.T) {
	now := time.Date(2026, 7, 22, 3, 0, 0, 0, time.UTC)
	const staleAfter = 10 * time.Minute
	tests := []struct {
		name        string
		run         model.MallWeatherFetchRun
		attempt     *model.MallWeatherFetchAttempt
		want        FetchAttemptDisposition
		wantRecover bool
		wantError   bool
	}{
		{name: "pending is acquired", run: fetchRunState(0, "pending"), want: FetchAttemptDispositionAcquired},
		{
			name: "failed is acquired", run: fetchRunState(1, "failed"),
			attempt: fetchAttemptState(1, "transport_failed", now.Add(-time.Minute)), want: FetchAttemptDispositionAcquired,
		},
		{
			name: "success is terminal", run: fetchRunState(1, "success"),
			attempt: fetchAttemptState(1, "success", now.Add(-time.Minute)), want: FetchAttemptDispositionTerminal,
		},
		{
			name: "partial success is terminal", run: fetchRunState(1, "partial_success"),
			attempt: fetchAttemptState(1, "partial_success", now.Add(-time.Minute)), want: FetchAttemptDispositionTerminal,
		},
		{
			name: "fresh running attempt is busy", run: fetchRunState(2, "running"),
			attempt: fetchAttemptState(2, "running", now.Add(-time.Minute)), want: FetchAttemptDispositionBusy,
		},
		{
			name: "stale running attempt is recovered", run: fetchRunState(2, "running"),
			attempt: fetchAttemptState(2, "running", now.Add(-staleAfter)), want: FetchAttemptDispositionAcquired, wantRecover: true,
		},
		{name: "running without matching attempt is rejected", run: fetchRunState(2, "running"), wantError: true},
		{name: "unknown state is rejected", run: fetchRunState(1, "deleted"), wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, recoverInterrupted, err := classifyFetchAttemptStart(&test.run, test.attempt, now, staleAfter)
			if (err != nil) != test.wantError {
				t.Fatalf("classifyFetchAttemptStart() error=%v wantError=%v", err, test.wantError)
			}
			if err == nil && (got != test.want || recoverInterrupted != test.wantRecover) {
				t.Fatalf("classifyFetchAttemptStart()=(%v,%v) want=(%v,%v)", got, recoverInterrupted, test.want, test.wantRecover)
			}
		})
	}
}

func fetchRunState(attemptCount int, status string) model.MallWeatherFetchRun {
	return model.MallWeatherFetchRun{BaseModel: model.BaseModel{ID: 7}, AttemptCount: attemptCount, Status: status}
}

func fetchAttemptState(attemptNo int, status string, startedAt time.Time) *model.MallWeatherFetchAttempt {
	return &model.MallWeatherFetchAttempt{
		BaseModel: model.BaseModel{ID: 11}, FetchRunID: 7, AttemptNo: attemptNo, StartedAt: startedAt, Status: status,
	}
}

func TestBuildHourlyQuery(t *testing.T) {
	start := time.Date(2026, 7, 17, 0, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	end := start.Add(72 * time.Hour)
	asOf := start.Add(10 * time.Hour)
	cursor := start.Add(12 * time.Hour)
	issuedCursor := cursor.Add(-time.Hour)

	tests := []struct {
		name         string
		query        HourlyQuery
		contains     []string
		wantArgCount int
		wantError    bool
	}{
		{
			name:         "latest versions use window function",
			query:        HourlyQuery{MallID: 1, StartUTC: start, EndUTC: end, Latest: true, Limit: 200},
			contains:     []string{"ROW_NUMBER() OVER", "PARTITION BY w.forecast_time_utc", "version_rank = 1", "LIMIT ?"},
			wantArgCount: 4,
		},
		{
			name:         "as of and cursor stay parameterized",
			query:        HourlyQuery{MallID: 1, StartUTC: start, EndUTC: end, AsOfUTC: &asOf, QualityStatus: "valid", AfterForecastTime: &cursor, AfterID: 99, Limit: 900},
			contains:     []string{"w.issued_at_utc <= ?", "w.quality_status = ?", "ranked.forecast_time_utc > ?", "ROW_NUMBER() OVER"},
			wantArgCount: 9,
		},
		{
			name:         "all versions omit ranking",
			query:        HourlyQuery{MallID: 1, StartUTC: start, EndUTC: end},
			contains:     []string{"SELECT w.*", "ORDER BY w.forecast_time_utc ASC"},
			wantArgCount: 4,
		},
		{
			name:         "version history cursor includes issued at",
			query:        HourlyQuery{MallID: 1, StartUTC: start, EndUTC: end, AfterForecastTime: &cursor, AfterIssuedAtUTC: &issuedCursor, AfterID: 7},
			contains:     []string{"w.issued_at_utc < ?", "w.issued_at_utc = ?", "ORDER BY w.forecast_time_utc ASC"},
			wantArgCount: 10,
		},
		{
			name:      "version history rejects incomplete cursor",
			query:     HourlyQuery{MallID: 1, StartUTC: start, EndUTC: end, AfterForecastTime: &cursor},
			wantError: true,
		},
		{
			name:      "invalid range",
			query:     HourlyQuery{MallID: 1, StartUTC: end, EndUTC: start},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			statement, args, err := buildHourlyQuery(tt.query)
			if (err != nil) != tt.wantError {
				t.Fatalf("buildHourlyQuery() error = %v, wantError %t", err, tt.wantError)
			}
			if err != nil {
				return
			}
			for _, fragment := range tt.contains {
				if !strings.Contains(statement, fragment) {
					t.Fatalf("query does not contain %q:\n%s", fragment, statement)
				}
			}
			if len(args) != tt.wantArgCount {
				t.Fatalf("argument count = %d, want %d", len(args), tt.wantArgCount)
			}
			if strings.Contains(statement, "valid") || strings.Contains(statement, "2026-") {
				t.Fatalf("query interpolated a caller value:\n%s", statement)
			}
		})
	}
}
