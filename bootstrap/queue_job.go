package bootstrap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	redisUsername := global.Credentials.QueueJobRedisUsername()
	redisPassword := global.Credentials.QueueJobRedisPassword()
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
	if config.GetBool("cfg.mall_weather.enabled") {
		geocodeProcessor, err := data_svc.NewMallGeocodeProcessor()
		if err != nil {
			console.Exit("Mall Geocode Worker Init Failed %v", err)
		}
		mux.HandleFunc(job.TypeMallGeocode, newMallGeocodeHandler(geocodeProcessor))

		weatherProcessor, err := data_svc.NewMallWeatherProcessor()
		if err != nil {
			console.Exit("Mall Weather Worker Init Failed %v", err)
		}
		weatherHandler := newMallWeatherHandler(weatherProcessor)
		for _, taskType := range job.MallWeatherFetchTaskTypes() {
			mux.HandleFunc(taskType, weatherHandler)
		}
	}

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

	startMallWeatherOutboxDispatcher()

}

type mallGeocodeProcessor interface {
	Process(ctx context.Context, payload job.MallGeocodeTaskPayload) error
}

type mallWeatherProcessor interface {
	Process(ctx context.Context, taskType string, payload job.MallTaskPayload) error
}

func newMallGeocodeHandler(processor mallGeocodeProcessor) asynq.HandlerFunc {
	return func(ctx context.Context, task *asynq.Task) error {
		if processor == nil {
			return fmt.Errorf("mall geocode handler: processor is not configured")
		}
		payload, err := job.DecodeMallGeocodeTaskPayload(task.Payload())
		if err != nil {
			return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
		}
		if err := processor.Process(ctx, payload); err != nil {
			var processError *data_svc.MallGeocodeProcessError
			if errors.As(err, &processError) && !processError.Retryable {
				return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
			}
			return err
		}
		return nil
	}
}

func newMallWeatherHandler(processor mallWeatherProcessor) asynq.HandlerFunc {
	return func(ctx context.Context, task *asynq.Task) error {
		if processor == nil {
			return fmt.Errorf("mall weather handler: processor is not configured")
		}
		if task == nil {
			return fmt.Errorf("%w: mall weather task is nil", asynq.SkipRetry)
		}
		payload, err := job.DecodeMallWeatherTaskPayload(task.Type(), task.Payload())
		if err != nil {
			return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
		}
		if err := processor.Process(ctx, task.Type(), payload); err != nil {
			var processError *data_svc.MallWeatherProcessError
			if errors.As(err, &processError) && processError != nil && !processError.Retryable {
				return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
			}
			return err
		}
		return nil
	}
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
