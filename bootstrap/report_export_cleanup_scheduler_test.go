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
	if enabled.calls != 2 || len(enabled.crons) != 2 || len(enabled.tasks) != 2 ||
		enabled.crons[0] != job.ReportExportCleanupCron || enabled.tasks[0] == nil || enabled.tasks[0].Type() != job.TypeReportExportCleanup ||
		enabled.crons[1] != job.ReportResultCleanupCron || enabled.tasks[1] == nil || enabled.tasks[1].Type() != job.TypeReportResultCleanup {
		t.Fatalf("enabled registrar=%+v", enabled)
	}
}

type fakeScheduledTaskRegistrar struct {
	calls int
	crons []string
	tasks []*asynq.Task
}

func (registrar *fakeScheduledTaskRegistrar) Register(cron string, task *asynq.Task, _ ...asynq.Option) (string, error) {
	registrar.calls++
	registrar.crons = append(registrar.crons, cron)
	registrar.tasks = append(registrar.tasks, task)
	return "report-export-cleanup", nil
}
