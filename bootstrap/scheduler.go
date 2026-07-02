package bootstrap

import (
	"context"
	"time"

	"gin-biz-web-api/global"
	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/job"
	"gin-biz-web-api/pkg/config"
	"gin-biz-web-api/pkg/console"

	"github.com/hibiken/asynq"
)

func setupScheduler() {
	console.Info("Scheduler Start ...")

	redisHost := config.GetString("cfg.queue_job.redis.host")
	redisPort := config.GetString("cfg.queue_job.redis.port")
	redisUsername := config.GetString("cfg.queue_job.redis.username")
	redisPassword := config.GetString("cfg.queue_job.redis.password")
	redisDB := config.GetInt("cfg.queue_job.redis.db")
	redisAddr := redisHost + ":" + redisPort

	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		console.Warning("Failed to load Asia/Shanghai timezone: %v", err)
		loc = time.FixedZone("CST", 8*60*60)
	}

	scheduler := asynq.NewScheduler(
		asynq.RedisClientOpt{
			Addr:     redisAddr,
			Username: redisUsername,
			Password: redisPassword,
			DB:       redisDB,
		},
		&asynq.SchedulerOpts{
			Location: loc,
		},
	)

	registerLegacyScheduledTasks(scheduler)

	registerDatabaseDeliveryTasks(scheduler)

	go func(scheduler *asynq.Scheduler) {
		if err := scheduler.Run(); err != nil {
			console.Warning("Scheduler Failed: %v", err)
			console.Exit("Scheduler Failed %v", err)
		}
	}(scheduler)

	global.QueueJobScheduler = scheduler

	console.Success("Scheduler started successfully")
}

func registerLegacyScheduledTasks(scheduler *asynq.Scheduler) {
	for _, definition := range job.ScheduledLegacyTaskDefinitions() {
		asynqTask, err := job.NewLegacyTask(definition.Code, definition.DefaultPayload)
		if err != nil {
			console.Warning("Failed to create legacy task %s: %v", definition.Code, err)
			continue
		}

		if _, err := scheduler.Register(definition.CronExpr, asynqTask); err != nil {
			console.Warning("Failed to register legacy task %s: %v", definition.Code, err)
		}
	}
}

func registerDatabaseDeliveryTasks(scheduler *asynq.Scheduler) {
	tasks, err := data_dao.NewDeliveryTaskDAO().FindEnabledScheduled(context.Background())
	if err != nil {
		console.Warning("Failed to load database delivery tasks: %v", err)
		return
	}

	for _, task := range tasks {
		asynqTask, err := job.NewDeliveryTaskRunTask(job.DeliveryTaskRunPayload{
			TaskID: task.ID,
		})
		if err != nil {
			console.Warning("Failed to create DeliveryTaskRun task %d: %v", task.ID, err)
			continue
		}

		if _, err := scheduler.Register(task.CronExpr, asynqTask); err != nil {
			console.Warning("Failed to register DeliveryTaskRun task %d: %v", task.ID, err)
		}
	}
}
