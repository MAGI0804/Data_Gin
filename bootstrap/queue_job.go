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
	redisUsername := config.GetString("cfg.queue_job.redis.username")
	redisPassword := config.GetString("cfg.queue_job.redis.password")
	redisDB := config.GetInt("cfg.queue_job.redis.db")
	redisAddr := redisHost + ":" + redisPort

	client := jobPkg.NewAsynqClient(redisAddr, redisUsername, redisPassword, redisDB)
	global.QueueJobClient = client

	reportWorkerEnabled := config.GetBool("cfg.queue_job.report_worker.enabled")
	queues := reportWorkerQueues(
		config.Get("cfg.queue_job.config_opt.queues").(map[string]int),
		reportWorkerEnabled,
		config.GetInt("cfg.queue_job.report_worker.queue_weight"),
	)
	server := jobPkg.NewAsynqServer(
		redisAddr,
		redisUsername,
		redisPassword,
		redisDB,
		config.GetInt("cfg.queue_job.config_opt.concurrency"),
		queues,
	)
	global.QueueJobServer = server

	mux := asynq.NewServeMux()
	mux.Use(jobLoggingMiddleware)

	addQueueJob(mux)
	if reportWorkerEnabled {
		reportProcessor := data_svc.NewReportRunProcessor()
		mux.HandleFunc(job.TypeReportRun, newReportRunHandler(reportProcessor))
		reportExportProcessor := data_svc.NewReportExportProcessor()
		mux.HandleFunc(job.TypeReportExport, newReportExportHandler(reportExportProcessor))
		reportExportCleaner := data_svc.NewReportExportCleaner()
		mux.HandleFunc(job.TypeReportExportCleanup, newReportExportCleanupHandler(reportExportCleaner))
		reportResultCleaner := data_svc.NewReportResultCleaner()
		mux.HandleFunc(job.TypeReportResultCleanup, newReportResultCleanupHandler(reportResultCleaner))
	}
	if cleanupTask, err := job.NewExcelMatchCleanupTask(); err != nil {
		console.Warning("Failed to create initial Excel match cleanup task: %v", err)
	} else if _, err := client.Enqueue(cleanupTask); err != nil && !errors.Is(err, asynq.ErrDuplicateTask) {
		console.Warning("Failed to enqueue initial Excel match cleanup task: %v", err)
	}
	if global.MallWeatherEnabledAtStartup {
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

		schedulePlanner, err := data_svc.NewMallWeatherSchedulePlanner()
		if err != nil {
			console.Exit("Mall Weather Scheduler Worker Init Failed %v", err)
		}
		mux.HandleFunc(job.TypeMallWeatherSchedule, newMallWeatherScheduleHandler(schedulePlanner))

		exportProcessor := data_svc.NewMallWeatherExportProcessor()
		mux.HandleFunc(job.TypeMallWeatherExport, newMallWeatherExportHandler(exportProcessor))

		exportCleaner := data_svc.NewMallWeatherExportCleaner()
		mux.HandleFunc(job.TypeMallWeatherExportCleanup, newMallWeatherExportCleanupHandler(exportCleaner))

		if err := registerMallWeatherFeishuWorker(
			mux,
			config.GetBool("cfg.mall_weather.feishu_enabled"),
			func() (mallWeatherFeishuProcessor, error) {
				return data_svc.NewMallWeatherFeishuProcessor()
			},
		); err != nil {
			console.Exit("Mall Weather Feishu Worker Init Failed %v", err)
		}
	}

	go func(mux *asynq.ServeMux, server *asynq.Server) {
		if err := server.Run(mux); err != nil {
			logger.Error("Queue Job Server Failed", zap.Error(err))
			console.Exit("Queue Job Server Failed %v", err)
		}
	}(mux, server)

	startOutboxDispatcher(reportWorkerEnabled)

}

type mallGeocodeProcessor interface {
	Process(ctx context.Context, payload job.MallGeocodeTaskPayload) error
}

type mallWeatherProcessor interface {
	Process(ctx context.Context, taskType string, payload job.MallTaskPayload) error
}

type mallWeatherSchedulePlanner interface {
	Plan(ctx context.Context, payload job.MallWeatherSchedulePayload) error
}

type mallWeatherExportProcessor interface {
	Process(ctx context.Context, jobID uint, retryAllowed bool) error
}

type mallWeatherFeishuProcessor interface {
	Process(ctx context.Context, pipelineRunID uint, retryAllowed bool) error
}

type mallWeatherExportCleaner interface {
	Cleanup(context.Context) (data_svc.MallWeatherExportCleanupResult, error)
}

type excelMatchCleanupRunner interface {
	CleanupExpiredJobs(context.Context) error
}

type reportRunProcessor interface {
	Process(context.Context, uint, bool) error
}

type reportExportProcessor interface {
	Process(context.Context, uint, bool) error
}

type reportExportCleaner interface {
	Cleanup(context.Context) (data_svc.ReportExportCleanupResult, error)
}

type reportResultCleaner interface {
	Cleanup(context.Context) (data_svc.ReportResultCleanupResult, error)
}

func reportWorkerQueues(configured map[string]int, enabled bool, weight int) map[string]int {
	queues := make(map[string]int, len(configured)+2)
	for name, value := range configured {
		if name != job.ReportQueueName && name != job.ReportExportQueueName {
			queues[name] = value
		}
	}
	if enabled {
		if weight <= 0 {
			weight = 2
		}
		queues[job.ReportQueueName] = weight
		queues[job.ReportExportQueueName] = weight
	}
	return queues
}

func newReportRunHandler(processor reportRunProcessor) asynq.HandlerFunc {
	return func(ctx context.Context, task *asynq.Task) error {
		if processor == nil {
			return fmt.Errorf("report run handler: processor is not configured")
		}
		if task == nil || task.Type() != job.TypeReportRun {
			return fmt.Errorf("%w: invalid report run task", asynq.SkipRetry)
		}
		payload, err := job.DecodeReportRunTaskPayload(task.Payload())
		if err != nil {
			return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
		}
		if err := processor.Process(ctx, payload.RunID, mallWeatherExportRetryAllowed(ctx)); err != nil {
			if errors.Is(err, data_svc.ErrReportRunProcessNonRetryable) {
				return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
			}
			return err
		}
		return nil
	}
}

func newReportExportCleanupHandler(cleaner reportExportCleaner) asynq.HandlerFunc {
	return func(ctx context.Context, task *asynq.Task) error {
		if cleaner == nil {
			return fmt.Errorf("report export cleanup handler: cleaner is not configured")
		}
		if task == nil || task.Type() != job.TypeReportExportCleanup {
			return fmt.Errorf("%w: invalid report export cleanup task", asynq.SkipRetry)
		}
		if err := job.DecodeReportExportCleanupTaskPayload(task.Payload()); err != nil {
			return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
		}
		_, err := cleaner.Cleanup(ctx)
		return err
	}
}

func newReportResultCleanupHandler(cleaner reportResultCleaner) asynq.HandlerFunc {
	return func(ctx context.Context, task *asynq.Task) error {
		if cleaner == nil {
			return fmt.Errorf("report result cleanup handler: cleaner is not configured")
		}
		if task == nil || task.Type() != job.TypeReportResultCleanup {
			return fmt.Errorf("%w: invalid report result cleanup task", asynq.SkipRetry)
		}
		if err := job.DecodeReportResultCleanupTaskPayload(task.Payload()); err != nil {
			return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
		}
		_, err := cleaner.Cleanup(ctx)
		return err
	}
}

func newReportExportHandler(processor reportExportProcessor) asynq.HandlerFunc {
	return func(ctx context.Context, task *asynq.Task) error {
		if processor == nil {
			return fmt.Errorf("report export handler: processor is not configured")
		}
		if task == nil || task.Type() != job.TypeReportExport {
			return fmt.Errorf("%w: invalid report export task", asynq.SkipRetry)
		}
		payload, err := job.DecodeReportExportTaskPayload(task.Payload())
		if err != nil {
			return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
		}
		if err := processor.Process(ctx, payload.ExportID, mallWeatherExportRetryAllowed(ctx)); err != nil {
			if errors.Is(err, data_svc.ErrReportExportProcessNonRetryable) {
				return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
			}
			return err
		}
		return nil
	}
}

func newExcelMatchCleanupHandler(cleaner excelMatchCleanupRunner) asynq.HandlerFunc {
	return func(ctx context.Context, task *asynq.Task) error {
		if cleaner == nil {
			return fmt.Errorf("excel match cleanup handler: cleaner is not configured")
		}
		if task == nil {
			return fmt.Errorf("%w: excel match cleanup task is nil", asynq.SkipRetry)
		}
		if err := job.DecodeExcelMatchCleanupTaskPayload(task.Payload()); err != nil {
			return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
		}
		return cleaner.CleanupExpiredJobs(ctx)
	}
}

func registerMallWeatherFeishuWorker(
	mux *asynq.ServeMux,
	enabled bool,
	processorFactory func() (mallWeatherFeishuProcessor, error),
) error {
	if !enabled {
		return nil
	}
	if mux == nil || processorFactory == nil {
		return fmt.Errorf("mall weather feishu worker: invalid registration configuration")
	}
	feishuProcessor, err := processorFactory()
	if err != nil {
		return err
	}
	mux.HandleFunc(job.TypeMallWeatherFeishu, newMallWeatherFeishuHandler(feishuProcessor))
	return nil
}

func newMallWeatherExportCleanupHandler(cleaner mallWeatherExportCleaner) asynq.HandlerFunc {
	return func(ctx context.Context, task *asynq.Task) error {
		if cleaner == nil {
			return fmt.Errorf("mall weather export cleanup handler: cleaner is not configured")
		}
		if task == nil {
			return fmt.Errorf("%w: mall weather export cleanup task is nil", asynq.SkipRetry)
		}
		if err := job.DecodeMallWeatherExportCleanupTaskPayload(task.Payload()); err != nil {
			return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
		}
		_, err := cleaner.Cleanup(ctx)
		if err != nil {
			return err
		}
		return nil
	}
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
			data_svc.RecordMallWeatherDeadLetterTask(task.Type(), data_svc.MallWeatherDeadLetterReasonInvalidPayload)
			return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
		}
		if err := processor.Process(ctx, task.Type(), payload); err != nil {
			var processError *data_svc.MallWeatherProcessError
			if errors.As(err, &processError) && processError != nil && !processError.Retryable {
				data_svc.RecordMallWeatherDeadLetterTask(task.Type(), data_svc.MallWeatherDeadLetterReasonPermanent)
				return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
			}
			return err
		}
		return nil
	}
}

func newMallWeatherScheduleHandler(planner mallWeatherSchedulePlanner) asynq.HandlerFunc {
	return func(ctx context.Context, task *asynq.Task) error {
		if planner == nil {
			return fmt.Errorf("mall weather schedule handler: planner is not configured")
		}
		if task == nil || task.Type() != job.TypeMallWeatherSchedule {
			recordMallWeatherInvalidTaskDeadLetter(task)
			return fmt.Errorf("%w: invalid mall weather schedule task", asynq.SkipRetry)
		}
		payload, err := job.DecodeMallWeatherSchedulePayload(task.Payload())
		if err != nil {
			data_svc.RecordMallWeatherDeadLetterTask(task.Type(), data_svc.MallWeatherDeadLetterReasonInvalidPayload)
			return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
		}
		return planner.Plan(ctx, payload)
	}
}

func newMallWeatherExportHandler(processor mallWeatherExportProcessor) asynq.HandlerFunc {
	return func(ctx context.Context, task *asynq.Task) error {
		if processor == nil {
			return fmt.Errorf("mall weather export handler: processor is not configured")
		}
		if task == nil || task.Type() != job.TypeMallWeatherExport {
			recordMallWeatherInvalidTaskDeadLetter(task)
			return fmt.Errorf("%w: invalid mall weather export task", asynq.SkipRetry)
		}
		payload, err := job.DecodeMallWeatherExportTaskPayload(task.Payload())
		if err != nil {
			data_svc.RecordMallWeatherDeadLetterTask(task.Type(), data_svc.MallWeatherDeadLetterReasonInvalidPayload)
			return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
		}
		if err := processor.Process(ctx, payload.ExportJobID, mallWeatherExportRetryAllowed(ctx)); err != nil {
			if errors.Is(err, data_svc.ErrMallWeatherExportProcessNonRetryable) {
				data_svc.RecordMallWeatherDeadLetterTask(task.Type(), data_svc.MallWeatherDeadLetterReasonPermanent)
				return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
			}
			return err
		}
		return nil
	}
}

func newMallWeatherFeishuHandler(processor mallWeatherFeishuProcessor) asynq.HandlerFunc {
	return func(ctx context.Context, task *asynq.Task) error {
		if processor == nil {
			return fmt.Errorf("mall weather feishu handler: processor is not configured")
		}
		if task == nil || task.Type() != job.TypeMallWeatherFeishu {
			recordMallWeatherInvalidTaskDeadLetter(task)
			return fmt.Errorf("%w: invalid mall weather feishu task", asynq.SkipRetry)
		}
		payload, err := job.DecodeMallWeatherFeishuTaskPayload(task.Payload())
		if err != nil {
			data_svc.RecordMallWeatherDeadLetterTask(task.Type(), data_svc.MallWeatherDeadLetterReasonInvalidPayload)
			return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
		}
		if err := processor.Process(ctx, payload.PipelineRunID, mallWeatherExportRetryAllowed(ctx)); err != nil {
			if errors.Is(err, data_svc.ErrMallWeatherFeishuProcessNonRetryable) {
				data_svc.RecordMallWeatherDeadLetterTask(task.Type(), data_svc.MallWeatherDeadLetterReasonPermanent)
				return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
			}
			return err
		}
		return nil
	}
}

func recordMallWeatherInvalidTaskDeadLetter(task *asynq.Task) {
	if task == nil {
		return
	}
	data_svc.RecordMallWeatherDeadLetterTask(task.Type(), data_svc.MallWeatherDeadLetterReasonInvalidPayload)
}

func mallWeatherExportRetryAllowed(ctx context.Context) bool {
	retryCount, retryOK := asynq.GetRetryCount(ctx)
	maxRetry, maxOK := asynq.GetMaxRetry(ctx)
	return !retryOK || !maxOK || retryCount < maxRetry
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
	mux.HandleFunc(job.TypeExcelMatchCleanup, newExcelMatchCleanupHandler(data_svc.NewExcelMatchJobService()))
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
	timeFilter, err := job.ResolveYouzanDistributionOrderTimeFilter(payload)
	if err != nil {
		return err
	}
	logger.Info(
		"开始拉取有赞分销订单",
		zap.String("time_filter", string(timeFilter)),
		zap.String("start_time", startTime),
		zap.String("end_time", endTime),
	)
	result, err := data_svc.NewYouzanDistributionOrderService().SyncRange(ctx, timeFilter, startTime, endTime)
	if err != nil {
		logger.Error(
			"有赞分销订单拉取失败",
			zap.String("time_filter", string(timeFilter)),
			zap.String("start_time", startTime),
			zap.String("end_time", endTime),
			zap.Error(err),
		)
		return err
	}
	logger.Info(
		"有赞分销订单拉取完成",
		zap.String("time_filter", string(timeFilter)),
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
