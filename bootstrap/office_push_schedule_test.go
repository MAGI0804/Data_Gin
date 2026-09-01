package bootstrap

import (
	"context"
	"errors"
	"testing"

	"gin-biz-web-api/job"

	"github.com/hibiken/asynq"
)

type fakeOfficePushSchedulePlanner struct {
	calls int
	err   error
}

func (planner *fakeOfficePushSchedulePlanner) Plan(context.Context) error {
	planner.calls++
	return planner.err
}

func TestOfficePushScheduleHandlerPlansStrictTask(t *testing.T) {
	planner := &fakeOfficePushSchedulePlanner{}
	task, err := job.NewOfficePushScheduleTask()
	if err != nil {
		t.Fatalf("NewOfficePushScheduleTask() error = %v", err)
	}
	if err := newOfficePushScheduleHandler(planner).ProcessTask(t.Context(), task); err != nil {
		t.Fatalf("ProcessTask() error = %v", err)
	}
	if planner.calls != 1 {
		t.Fatalf("planner calls = %d", planner.calls)
	}
}

func TestOfficePushScheduleHandlerSkipsInvalidTask(t *testing.T) {
	for _, task := range []*asynq.Task{
		nil,
		asynq.NewTask(job.TypeOfficePushSchedule, []byte(`{"secret":"x"}`)),
		asynq.NewTask(job.TypeOfficePush, []byte(`{}`)),
	} {
		err := newOfficePushScheduleHandler(&fakeOfficePushSchedulePlanner{}).ProcessTask(t.Context(), task)
		if !errors.Is(err, asynq.SkipRetry) {
			t.Fatalf("ProcessTask() error = %v", err)
		}
	}
}

func TestRegisterOfficePushScheduledTasks(t *testing.T) {
	registrar := &fakeScheduledTaskRegistrar{}
	registerOfficePushScheduledTasks(registrar)
	if registrar.calls != 1 || len(registrar.crons) != 1 || registrar.crons[0] != job.OfficePushScheduleCron ||
		len(registrar.tasks) != 1 || registrar.tasks[0].Type() != job.TypeOfficePushSchedule {
		t.Fatalf("registrar = %+v", registrar)
	}
}
