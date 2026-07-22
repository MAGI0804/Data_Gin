package data_svc

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/job"
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/database"
)

const mallWeatherSchedulePageSize = 200

type mallWeatherScheduleStore interface {
	ListEnabledMalls(ctx context.Context, afterID uint, limit int) ([]model.Mall, error)
	CreateOutboxes(ctx context.Context, rows []model.AsyncJobOutbox) (int64, error)
}

type gormMallWeatherScheduleStore struct{}

func (gormMallWeatherScheduleStore) ListEnabledMalls(ctx context.Context, afterID uint, limit int) ([]model.Mall, error) {
	return data_dao.NewMallDAO(database.DB).ListEnabledWeatherAfterID(ctx, afterID, limit)
}

func (gormMallWeatherScheduleStore) CreateOutboxes(ctx context.Context, rows []model.AsyncJobOutbox) (int64, error) {
	return data_dao.NewAsyncJobOutboxDAO(database.DB).CreateBatchIgnoreTaskConflicts(ctx, rows)
}

type MallWeatherSchedulePlanner struct {
	store    mallWeatherScheduleStore
	now      func() time.Time
	location *time.Location
}

func NewMallWeatherSchedulePlanner() (*MallWeatherSchedulePlanner, error) {
	if database.DB == nil {
		return nil, fmt.Errorf("mall weather scheduler: database is unavailable")
	}
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		location = time.FixedZone("CST", 8*60*60)
	}
	return newMallWeatherSchedulePlanner(gormMallWeatherScheduleStore{}, time.Now, location)
}

func newMallWeatherSchedulePlanner(store mallWeatherScheduleStore, now func() time.Time, location *time.Location) (*MallWeatherSchedulePlanner, error) {
	if store == nil || now == nil || location == nil {
		return nil, fmt.Errorf("mall weather scheduler: invalid configuration")
	}
	return &MallWeatherSchedulePlanner{store: store, now: now, location: location}, nil
}

func (planner *MallWeatherSchedulePlanner) Plan(ctx context.Context, payload job.MallWeatherSchedulePayload) error {
	if planner == nil || ctx == nil {
		return fmt.Errorf("mall weather scheduler: invalid planner")
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("mall weather scheduler: encode schedule payload: %w", err)
	}
	payload, err = job.DecodeMallWeatherSchedulePayload(encoded)
	if err != nil {
		return err
	}
	scheduledAt := planner.now().UTC()
	var afterID uint
	for {
		malls, err := planner.store.ListEnabledMalls(ctx, afterID, mallWeatherSchedulePageSize)
		if err != nil {
			return fmt.Errorf("mall weather scheduler: list enabled malls: %w", err)
		}
		rows := make([]model.AsyncJobOutbox, 0, len(malls))
		for index := range malls {
			mall := &malls[index]
			if mall.DetailProfile != payload.DetailProfile || mall.WeatherProvider != "caiyun" {
				continue
			}
			row, err := planner.outboxForMall(payload.TaskType, mall.ID, scheduledAt)
			if err != nil {
				return err
			}
			rows = append(rows, row)
		}
		if _, err := planner.store.CreateOutboxes(ctx, rows); err != nil {
			return fmt.Errorf("mall weather scheduler: store outboxes: %w", err)
		}
		if len(malls) < mallWeatherSchedulePageSize {
			return nil
		}
		afterID = malls[len(malls)-1].ID
		if afterID == 0 {
			return fmt.Errorf("mall weather scheduler: invalid mall page cursor")
		}
	}
}

func (planner *MallWeatherSchedulePlanner) outboxForMall(taskType string, mallID uint, scheduledAt time.Time) (model.AsyncJobOutbox, error) {
	if mallID == 0 || scheduledAt.IsZero() {
		return model.AsyncJobOutbox{}, fmt.Errorf("mall weather scheduler: invalid task identity")
	}
	local := scheduledAt.In(planner.location)
	var window string
	switch taskType {
	case job.TypeMallWeatherFast:
		window = fmt.Sprintf("fast:%d:%s", mallID, local.Format("200601021504"))
	case job.TypeMallWeatherFull:
		window = fmt.Sprintf("full:%d:%s", mallID, local.Format("2006010215"))
	case job.TypeMallWeatherLifeIndex:
		window = fmt.Sprintf("life:%d:%s", mallID, local.Format("2006010215"))
	default:
		return model.AsyncJobOutbox{}, fmt.Errorf("mall weather scheduler: unsupported task type")
	}
	payloadJSON, err := json.Marshal(job.MallTaskPayload{MallID: mallID, TaskWindow: window})
	if err != nil {
		return model.AsyncJobOutbox{}, fmt.Errorf("mall weather scheduler: encode task payload: %w", err)
	}
	return model.AsyncJobOutbox{
		TaskKey: "mall:weather:" + window, TaskType: taskType, PayloadJSON: model.JSONText(payloadJSON),
		QueueName: job.MallWeatherQueueName, AvailableAt: scheduledAt.UTC(),
	}, nil
}
