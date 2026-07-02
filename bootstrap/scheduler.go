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

	youzanSyncTask, err := job.NewYouzanSyncTask(job.YouzanSyncPayload{})
	if err != nil {
		console.Warning("Failed to create YouzanSync task: %v", err)
	} else {
		_, err = scheduler.Register("@every 1m", youzanSyncTask)
		if err != nil {
			console.Warning("Failed to register YouzanSync task: %v", err)
		}
	}

	youzanReturnTask, err := job.NewYouzanReturnTask(job.YouzanReturnPayload{})
	if err != nil {
		console.Warning("Failed to create YouzanReturn task: %v", err)
	} else {
		_, err = scheduler.Register("@every 1m", youzanReturnTask)
		if err != nil {
			console.Warning("Failed to register YouzanReturn task: %v", err)
		}
	}

	youzanSalesSyncTask, err := job.NewYouzanSalesSyncTask(job.YouzanSalesSyncPayload{})
	if err != nil {
		console.Warning("Failed to create YouzanSalesSync task: %v", err)
	} else {
		_, err = scheduler.Register("@every 1m", youzanSalesSyncTask)
		if err != nil {
			console.Warning("Failed to register YouzanSalesSync task: %v", err)
		}
	}

	youzanRefundSyncTask, err := job.NewYouzanRefundSyncTask(job.YouzanRefundSyncPayload{})
	if err != nil {
		console.Warning("Failed to create YouzanRefundSync task: %v", err)
	} else {
		_, err = scheduler.Register("@every 1m", youzanRefundSyncTask)
		if err != nil {
			console.Warning("Failed to register YouzanRefundSync task: %v", err)
		}
	}

	salesSyncTask, err := job.NewSalesSyncTask(job.SalesSyncPayload{
		ShopCode:     config.GetString("cfg.henglong.sync.shop_code"),
		Status:       config.GetString("cfg.henglong.sync.status", "70"),
		StoreCode:    config.GetString("cfg.henglong.sync.store_code"),
		MallItemCode: config.GetString("cfg.henglong.sync.mall_item_code"),
	})
	if err != nil {
		console.Warning("Failed to create SalesSync task: %v", err)
	} else {
		_, err = scheduler.Register("@every 1m", salesSyncTask)
		if err != nil {
			console.Warning("Failed to register SalesSync task: %v", err)
		}
	}

	xianOrderSyncTask, err := job.NewXianOrderSyncTask(job.XianOrderSyncPayload{
		ShopCode: config.GetString("cfg.xian.sync.shop_code"),
		Status:   config.GetString("cfg.xian.sync.status", "70"),
	})
	if err != nil {
		console.Warning("Failed to create XianOrderSync task: %v", err)
	} else {
		_, err = scheduler.Register("@every 1m", xianOrderSyncTask)
		if err != nil {
			console.Warning("Failed to register XianOrderSync task: %v", err)
		}
	}

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
