package data_svc

import (
	"context"
	"testing"
	"time"

	"gin-biz-web-api/job"
	"gin-biz-web-api/model"
)

func TestMallWeatherSchedulePlannerCreatesProfileOutboxes(t *testing.T) {
	store := &fakeMallWeatherScheduleStore{malls: []model.Mall{
		{BaseModel: model.BaseModel{ID: 7}, DetailProfile: "full", WeatherProvider: "caiyun"},
		{BaseModel: model.BaseModel{ID: 8}, DetailProfile: "standard", WeatherProvider: "caiyun"},
	}}
	scheduledAt := time.Date(2026, 7, 22, 3, 7, 30, 0, time.UTC)
	planner, err := newMallWeatherSchedulePlanner(store, func() time.Time { return scheduledAt }, time.FixedZone("CST", 8*60*60))
	if err != nil {
		t.Fatalf("newMallWeatherSchedulePlanner() error=%v", err)
	}
	if err := planner.Plan(context.Background(), job.MallWeatherSchedulePayload{
		TaskType: job.TypeMallWeatherFull, DetailProfile: "full",
	}); err != nil {
		t.Fatalf("Plan() error=%v", err)
	}
	if len(store.rows) != 1 {
		t.Fatalf("outboxes=%+v", store.rows)
	}
	row := store.rows[0]
	if row.TaskKey != "mall:weather:full:7:2026072211" || row.TaskType != job.TypeMallWeatherFull ||
		string(row.PayloadJSON) != `{"mall_id":7,"task_window":"full:7:2026072211"}` || !row.AvailableAt.Equal(scheduledAt) {
		t.Fatalf("outbox=%+v", row)
	}
}

func TestMallWeatherSchedulePlannerUsesStableBusinessWindows(t *testing.T) {
	planner, err := newMallWeatherSchedulePlanner(&fakeMallWeatherScheduleStore{}, time.Now, time.UTC)
	if err != nil {
		t.Fatalf("newMallWeatherSchedulePlanner() error=%v", err)
	}
	scheduledAt := time.Date(2026, 7, 22, 3, 9, 59, 0, time.UTC)
	fast, err := planner.outboxForMall(job.TypeMallWeatherFast, 7, scheduledAt)
	if err != nil {
		t.Fatalf("outboxForMall(fast) error=%v", err)
	}
	if fast.TaskKey != "mall:weather:fast:7:202607220309" {
		t.Fatalf("fast task key=%s", fast.TaskKey)
	}
	life, err := planner.outboxForMall(job.TypeMallWeatherLifeIndex, 7, scheduledAt)
	if err != nil {
		t.Fatalf("outboxForMall(life) error=%v", err)
	}
	if life.TaskKey != "mall:weather:life:7:2026072203" {
		t.Fatalf("life task key=%s", life.TaskKey)
	}
}

type fakeMallWeatherScheduleStore struct {
	malls []model.Mall
	rows  []model.AsyncJobOutbox

	listErr   error
	createErr error
}

func (store *fakeMallWeatherScheduleStore) ListEnabledMalls(_ context.Context, afterID uint, limit int) ([]model.Mall, error) {
	if store.listErr != nil {
		return nil, store.listErr
	}
	rows := make([]model.Mall, 0, limit)
	for _, mall := range store.malls {
		if mall.ID > afterID && len(rows) < limit {
			rows = append(rows, mall)
		}
	}
	return rows, nil
}

func (store *fakeMallWeatherScheduleStore) CreateOutboxes(_ context.Context, rows []model.AsyncJobOutbox) (int64, error) {
	if store.createErr != nil {
		return 0, store.createErr
	}
	store.rows = append(store.rows, rows...)
	return int64(len(rows)), nil
}
