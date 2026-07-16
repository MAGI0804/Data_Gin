package bootstrap

import (
	"context"
	"encoding/json"
	"time"

	"gin-biz-web-api/global"
	"gin-biz-web-api/internal/service/data_svc"
	"gin-biz-web-api/job"
	"gin-biz-web-api/pkg/config"
	"gin-biz-web-api/pkg/console"
	jobPkg "gin-biz-web-api/pkg/job"
	"gin-biz-web-api/pkg/logger"

	"github.com/hibiken/asynq"
	"go.uber.org/zap"
)

func setupQueueJob() {

	console.Info("Queue Job Start ...")

	redisHost := config.GetString("cfg.queue_job.redis.host")
	redisPort := config.GetString("cfg.queue_job.redis.port")
	redisUsername := config.GetString("cfg.queue_job.redis.username")
	redisPassword := config.GetString("cfg.queue_job.redis.password")
	redisDB := config.GetInt("cfg.queue_job.redis.db")
	redisAddr := redisHost + ":" + redisPort

	client := jobPkg.NewAsynqClient(redisAddr, redisUsername, redisPassword, redisDB)
	global.QueueJobClient = client

	server := jobPkg.NewAsynqServer(
		redisAddr,
		redisUsername,
		redisPassword,
		redisDB,
		config.GetInt("cfg.queue_job.config_opt.concurrency"),
		config.Get("cfg.queue_job.config_opt.queues").(map[string]int),
	)
	global.QueueJobServer = server

	mux := asynq.NewServeMux()
	mux.Use(jobLoggingMiddleware)

	addQueueJob(mux)

	go func(mux *asynq.ServeMux, server *asynq.Server) {
		if err := server.Run(mux); err != nil {
			logger.Error("Queue Job Server Failed", zap.Error(err))
			console.Exit("Queue Job Server Failed %v", err)
		}
	}(mux, server)

	go func() {
		if err := data_svc.NewExcelMatchJobService().CleanupExpiredJobs(context.Background()); err != nil {
			logger.Error("Excel Match Job Cleanup Failed", zap.Error(err))
		}
	}()

}

// addQueueJob 添加异步队列任务
func addQueueJob(mux *asynq.ServeMux) {
	mux.HandleFunc(job.TypeFoo, job.HandFooTask)
	mux.HandleFunc(job.TypeDataProcess, job.HandleDataProcessTask)
	mux.HandleFunc(job.TypeYouzanSync, job.HandleYouzanSyncTask)
	mux.HandleFunc(job.TypeYouzanReturn, job.HandleYouzanReturnTask)
	mux.HandleFunc(job.TypeSalesSync, job.HandleSalesSyncTask)
	//mux.HandleFunc(job.TypeYouzanSalesSync, job.HandleYouzanSalesSyncTask)
	//mux.HandleFunc(job.TypeYouzanRefundSync, job.HandleYouzanRefundSyncTask)
	mux.HandleFunc(job.TypeXianOrderSync, job.HandleXianOrderSyncTask)
	mux.HandleFunc(job.TypeDeliveryTaskRun, handleDeliveryTaskRun)
	mux.HandleFunc(job.TypeExcelMatchExport, handleExcelMatchExport)
	mux.HandleFunc(job.TypeBojunOrderFetch, handleBojunOrderFetch)
	mux.HandleFunc(job.TypeYouzanDistributionOrderSync, handleYouzanDistributionOrderSync)
}

func handleDeliveryTaskRun(ctx context.Context, task *asynq.Task) error {
	var payload job.DeliveryTaskRunPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return err
	}

	_, err := data_svc.NewDeliveryService().RunDeliveryTask(ctx, payload.TaskID)
	return err
}

func handleExcelMatchExport(ctx context.Context, task *asynq.Task) error {
	var payload job.ExcelMatchExportPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return err
	}

	return data_svc.NewExcelMatchJobService().ProcessJob(ctx, payload.JobID)
}

func handleBojunOrderFetch(ctx context.Context, task *asynq.Task) error {
	var payload job.BojunOrderFetchPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return err
	}

	_, err := data_svc.NewBojunOrderService().SyncOrders(ctx, payload.StartTime, payload.EndTime)
	return err
}

func handleYouzanDistributionOrderSync(ctx context.Context, task *asynq.Task) error {
	var payload job.YouzanDistributionOrderSyncPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return err
	}

	startTime, endTime := job.ResolveYouzanDistributionOrderRange(payload, time.Now())
	logger.Info(
		"开始拉取有赞分销订单",
		zap.String("start_time", startTime),
		zap.String("end_time", endTime),
	)
	result, err := data_svc.NewYouzanDistributionOrderService().SyncRange(ctx, startTime, endTime)
	if err != nil {
		logger.Error(
			"有赞分销订单拉取失败",
			zap.String("start_time", startTime),
			zap.String("end_time", endTime),
			zap.Error(err),
		)
		return err
	}
	logger.Info(
		"有赞分销订单拉取完成",
		zap.String("start_time", startTime),
		zap.String("end_time", endTime),
		zap.Int("fetch_pages", result.FetchPages),
		zap.Int("total_count", result.TotalCount),
		zap.Int("saved_count", result.SavedCount),
		zap.Int("existing_count", result.ExistingCount),
		zap.Int("failed_count", result.FailedCount),
	)
	return nil
}

// jobLoggingMiddleware 异步任务执行日志中间件
func jobLoggingMiddleware(h asynq.Handler) asynq.Handler {
	return asynq.HandlerFunc(func(ctx context.Context, task *asynq.Task) error {
		start := time.Now()
		logger.Info(
			"Start processing",
			zap.String("Type", task.Type()),
			zap.ByteString("Payload", task.Payload()),
		)
		err := h.ProcessTask(ctx, task)
		if err != nil {
			return err
		}
		logger.Info(
			"Finished processing",
			zap.String("Type", task.Type()),
			zap.Duration("Elapsed Time", time.Since(start)),
		)
		return nil
	})
}
