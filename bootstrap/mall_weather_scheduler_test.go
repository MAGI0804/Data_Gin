package bootstrap

import (
	"context"
	"errors"
	"testing"

	"gin-biz-web-api/job"

	"github.com/hibiken/asynq"
)

type fakeMallWeatherSchedulePlanner struct {
	payload job.MallWeatherSchedulePayload
	err     error
}

func (planner *fakeMallWeatherSchedulePlanner) Plan(_ context.Context, payload job.MallWeatherSchedulePayload) error {
	planner.payload = payload
	return planner.err
}

func TestMallWeatherScheduleHandlerDecodesAndPlans(t *testing.T) {
	planner := &fakeMallWeatherSchedulePlanner{}
	task, err := job.NewMallWeatherScheduleTask(job.MallWeatherSchedulePayload{
		TaskType: job.TypeMallWeatherFast, DetailProfile: "full",
	})
	if err != nil {
		t.Fatalf("NewMallWeatherScheduleTask() error=%v", err)
	}
	if err := newMallWeatherScheduleHandler(planner).ProcessTask(context.Background(), task); err != nil {
		t.Fatalf("ProcessTask() error=%v", err)
	}
	if planner.payload.TaskType != job.TypeMallWeatherFast || planner.payload.DetailProfile != "full" {
		t.Fatalf("payload=%+v", planner.payload)
	}
}

func TestMallWeatherScheduleHandlerSkipsInvalidPayload(t *testing.T) {
	err := newMallWeatherScheduleHandler(&fakeMallWeatherSchedulePlanner{}).ProcessTask(
		context.Background(),
		asynq.NewTask(job.TypeMallWeatherSchedule, []byte(`{"task_type":"unknown"}`)),
	)
	if !errors.Is(err, asynq.SkipRetry) {
		t.Fatalf("ProcessTask() error=%v", err)
	}
}
