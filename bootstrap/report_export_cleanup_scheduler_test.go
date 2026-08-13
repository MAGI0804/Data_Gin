package bootstrap

import (
	"testing"

	"gin-biz-web-api/job"

	"github.com/hibiken/asynq"
)

func TestRegisterReportScheduledTasksHonorsWorkerFlag(t *testing.T) {
	disabled := &fakeScheduledTaskRegistrar{}
	registerReportScheduledTasks(disabled, false)
	if disabled.calls != 0 {
		t.Fatalf("disabled registrar calls=%d", disabled.calls)
	}

	enabled := &fakeScheduledTaskRegistrar{}
	registerReportScheduledTasks(enabled, true)
	if enabled.calls != 1 || enabled.cron != job.ReportExportCleanupCron || enabled.task == nil || enabled.task.Type() != job.TypeReportExportCleanup {
		t.Fatalf("enabled registrar=%+v", enabled)
	}
}

type fakeScheduledTaskRegistrar struct {
	calls int
	cron  string
	task  *asynq.Task
}

func (registrar *fakeScheduledTaskRegistrar) Register(cron string, task *asynq.Task, _ ...asynq.Option) (string, error) {
	registrar.calls++
	registrar.cron = cron
	registrar.task = task
	return "report-export-cleanup", nil
}
