package routers

import (
	"gin-biz-web-api/internal/controller/data_ctrl"
	"gin-biz-web-api/internal/middleware"

	"github.com/gin-gonic/gin"
)

func apiData(api *gin.RouterGroup) {
	registerMallRoutes(api, data_ctrl.NewMallController())
	registerMallWeatherRoutes(api, data_ctrl.NewMallWeatherController())
	registerMallWeatherRefreshRoutes(api, data_ctrl.NewMallWeatherRefreshController())
	registerMallWeatherExportProfileRoutes(api, data_ctrl.NewMallWeatherExportProfileController())
	registerMallWeatherExportJobRoutes(api, data_ctrl.NewMallWeatherExportJobController())
	registerMallWeatherFeishuPushRoutes(api, data_ctrl.NewMallWeatherFeishuPushController())
	registerMallWeatherMetricsRoutes(api, data_ctrl.NewMallWeatherMetricsController())

	sourceGroup := api.Group("/v1/sources")
	sourceGroup.Use(middleware.AuthJWT())
	{
		sourceCtrl := data_ctrl.NewSourceController()
		sourceGroup.GET("", sourceCtrl.ListSources)
		sourceGroup.GET("/:id", sourceCtrl.GetSource)
		sourceGroup.POST("", sourceCtrl.CreateSource)
		sourceGroup.PUT("/:id", sourceCtrl.UpdateSource)
		sourceGroup.POST("/:id/test", sourceCtrl.TestSource)
		sourceGroup.POST("/:id/fetch", sourceCtrl.FetchSource)
	}

	transformGroup := api.Group("/v1/transform-rules")
	transformGroup.Use(middleware.AuthJWT())
	{
		transformCtrl := data_ctrl.NewTransformController()
		transformGroup.GET("", transformCtrl.ListRules)
		transformGroup.GET("/:id", transformCtrl.GetRule)
		transformGroup.POST("", transformCtrl.CreateRule)
		transformGroup.PUT("/:id", transformCtrl.UpdateRule)
		transformGroup.POST("/test", transformCtrl.TestRule)
	}

	rawRecordsGroup := api.Group("/v1/raw-records")
	rawRecordsGroup.Use(middleware.AuthJWT())
	{
		transformCtrl := data_ctrl.NewTransformController()
		rawRecordsGroup.POST("/:id/retransform", transformCtrl.RetransformRawRecord)
	}

	destinationGroup := api.Group("/v1/destinations")
	destinationGroup.Use(middleware.AuthJWT())
	{
		deliveryCtrl := data_ctrl.NewDeliveryController()
		destinationGroup.GET("", deliveryCtrl.ListDestinations)
		destinationGroup.GET("/:id", deliveryCtrl.GetDestination)
		destinationGroup.POST("", deliveryCtrl.CreateDestination)
		destinationGroup.PUT("/:id", deliveryCtrl.UpdateDestination)
		destinationGroup.POST("/:id/test", deliveryCtrl.TestDestination)
	}

	deliveryTaskGroup := api.Group("/v1/delivery-tasks")
	deliveryTaskGroup.Use(middleware.AuthJWT())
	{
		deliveryCtrl := data_ctrl.NewDeliveryController()
		deliveryTaskGroup.GET("", deliveryCtrl.ListTasks)
		deliveryTaskGroup.GET("/:id", deliveryCtrl.GetTask)
		deliveryTaskGroup.POST("", deliveryCtrl.CreateTask)
		deliveryTaskGroup.PUT("/:id", deliveryCtrl.UpdateTask)
		deliveryTaskGroup.POST("/:id/run", deliveryCtrl.RunTask)
	}

	deliveryLogGroup := api.Group("/v1/delivery-logs")
	deliveryLogGroup.Use(middleware.AuthJWT())
	{
		deliveryCtrl := data_ctrl.NewDeliveryController()
		deliveryLogGroup.GET("", deliveryCtrl.ListLogs)
		deliveryLogGroup.POST("/:id/retry", deliveryCtrl.RetryLog)
	}

	orderPushConfigGroup := api.Group("/v1/order-push-skip-config")
	orderPushConfigGroup.Use(middleware.AuthJWT())
	{
		orderPushConfigCtrl := data_ctrl.NewOrderPushConfigController()
		orderPushConfigGroup.GET("", orderPushConfigCtrl.GetSkipPolicy)
		orderPushConfigGroup.PUT("", orderPushConfigCtrl.SaveSkipPolicy)
	}

	runGroup := api.Group("/v1/runs")
	runGroup.Use(middleware.AuthJWT())
	{
		runCtrl := data_ctrl.NewRunController()
		runGroup.GET("", runCtrl.ListRuns)
	}

	pipelineGroup := api.Group("/v1/pipelines")
	pipelineGroup.Use(middleware.AuthJWT())
	{
		pipelineCtrl := data_ctrl.NewPipelineController()
		pipelineGroup.GET("", pipelineCtrl.ListPipelines)
		pipelineGroup.GET("/:id", pipelineCtrl.GetPipeline)
		pipelineGroup.POST("", pipelineCtrl.CreatePipeline)
		pipelineGroup.PUT("/:id", pipelineCtrl.UpdatePipeline)
		pipelineGroup.GET("/:id/stages", pipelineCtrl.ListStages)
		pipelineGroup.POST("/:id/stages", pipelineCtrl.CreateStage)
		pipelineGroup.GET("/:id/steps", pipelineCtrl.ListSteps)
		pipelineGroup.POST("/:id/steps", pipelineCtrl.CreateStep)
		pipelineGroup.PUT("/:id/steps/:step_id", pipelineCtrl.UpdateStep)
		pipelineGroup.GET("/:id/preview-json", pipelineCtrl.PreviewJSON)
		pipelineGroup.POST("/:id/run", pipelineCtrl.RunPipeline)
	}

	pipelineStageGroup := api.Group("/v1/pipeline-stages")
	pipelineStageGroup.Use(middleware.AuthJWT())
	{
		pipelineCtrl := data_ctrl.NewPipelineController()
		pipelineStageGroup.PUT("/:stage_id", pipelineCtrl.UpdateStage)
		pipelineStageGroup.POST("/:stage_id/steps", pipelineCtrl.CreateStageStep)
		pipelineStageGroup.PUT("/:stage_id/steps/:step_id", pipelineCtrl.UpdateStep)
		pipelineStageGroup.POST("/:stage_id/generate-config", pipelineCtrl.GenerateStageConfig)
		pipelineStageGroup.POST("/:stage_id/publish-config", pipelineCtrl.PublishStageConfig)
	}

	stepRunGroup := api.Group("/v1/pipeline-runs")
	stepRunGroup.Use(middleware.AuthJWT())
	{
		pipelineCtrl := data_ctrl.NewPipelineController()
		stepRunGroup.GET("/:id/steps", pipelineCtrl.ListStepRuns)
	}

	excelMatchJobGroup := api.Group("/v1/excel-match-jobs")
	excelMatchJobGroup.Use(middleware.AuthJWT())
	{
		excelMatchJobCtrl := data_ctrl.NewExcelMatchJobController()
		excelMatchJobGroup.GET("", excelMatchJobCtrl.ListJobs)
		excelMatchJobGroup.POST("", excelMatchJobCtrl.CreateJob)
		excelMatchJobGroup.GET("/models", excelMatchJobCtrl.ListModels)
		excelMatchJobGroup.POST("/preview", excelMatchJobCtrl.Preview)
		excelMatchJobGroup.POST("/uploads", excelMatchJobCtrl.CreateUploadSession)
		excelMatchJobGroup.POST("/uploads/:upload_id/chunks", excelMatchJobCtrl.UploadChunk)
		excelMatchJobGroup.POST("/uploads/:upload_id/complete", excelMatchJobCtrl.CompleteUpload)
		excelMatchJobGroup.GET("/schemes", excelMatchJobCtrl.ListSchemes)
		excelMatchJobGroup.POST("/schemes", excelMatchJobCtrl.SaveScheme)
		excelMatchJobGroup.DELETE("/schemes/:scheme_id", excelMatchJobCtrl.DeleteScheme)
		excelMatchJobGroup.GET("/:id", excelMatchJobCtrl.GetJob)
		excelMatchJobGroup.GET("/:id/download", excelMatchJobCtrl.Download)
		excelMatchJobGroup.POST("/:id/download", excelMatchJobCtrl.Download)
	}

	legacyTaskGroup := api.Group("/v1/legacy-tasks")
	legacyTaskGroup.Use(middleware.AuthJWT())
	{
		legacyTaskCtrl := data_ctrl.NewLegacyTaskController()
		legacyTaskGroup.GET("", legacyTaskCtrl.List)
		legacyTaskGroup.POST("/:code/run", legacyTaskCtrl.Run)
	}

	bojunOrderBackfillGroup := api.Group("/v1/bojun-order-backfill")
	bojunOrderBackfillGroup.Use(middleware.AuthJWT())
	{
		bojunOrderBackfillCtrl := data_ctrl.NewBojunOrderBackfillController()
		bojunOrderBackfillGroup.POST("/preview", bojunOrderBackfillCtrl.Preview)
		bojunOrderBackfillGroup.POST("/confirm", bojunOrderBackfillCtrl.Confirm)
	}

	youzanDistributionOrderBackfillGroup := api.Group("/v1/youzan-distribution-order-backfill")
	youzanDistributionOrderBackfillGroup.Use(middleware.AuthJWT())
	{
		youzanDistributionOrderBackfillCtrl := data_ctrl.NewYouzanDistributionOrderBackfillController()
		youzanDistributionOrderBackfillGroup.POST("/preview", youzanDistributionOrderBackfillCtrl.Preview)
		youzanDistributionOrderBackfillGroup.POST("/confirm", youzanDistributionOrderBackfillCtrl.Confirm)
	}

	legacyTransformRuleGroup := api.Group("/v1/legacy-transform-rules")
	legacyTransformRuleGroup.Use(middleware.AuthJWT())
	{
		legacyTaskCtrl := data_ctrl.NewLegacyTaskController()
		legacyTransformRuleGroup.GET("", legacyTaskCtrl.ListTransformRules)
	}

	dataGroup := api.Group("/v1/data")
	dataGroup.Use(middleware.AuthJWT()) // 需要认证
	{
		dataCtrl := data_ctrl.NewDataController()

		// 数据采集
		dataGroup.POST("/collect/:source_id", dataCtrl.CollectController.ManualCollect)
		dataGroup.GET("/collect/status/:job_id", dataCtrl.CollectController.CollectStatus)
		// 任务创建
		dataGroup.POST("/task/create/:source_id", dataCtrl.CollectController.CreateTask)

		// 数据接收
		dataGroup.POST("/ingest", dataCtrl.IngestController.IngestData)
		dataGroup.POST("/ingest/batch", dataCtrl.IngestController.IngestBatchData)
		dataGroup.POST("/ingest/raw", dataCtrl.IngestController.RawIngestData) // 接收原始格式数据

		// 数据查询
		dataGroup.GET("/raw", dataCtrl.QueryController.GetRawData)
		dataGroup.POST("/raw/list", dataCtrl.QueryController.GetRawDataList)
		dataGroup.GET("/processed", dataCtrl.QueryController.GetProcessedData)
		dataGroup.GET("/statistics", dataCtrl.QueryController.GetStatistics)
	}
}

func registerMallWeatherRoutes(api *gin.RouterGroup, weatherCtrl *data_ctrl.MallWeatherController) {
	weatherGroup := api.Group("/v1/malls")
	weatherGroup.Use(middleware.AuthJWT())
	{
		weatherGroup.GET("/:id/weather/overview", weatherCtrl.Overview)
		weatherGroup.GET("/:id/weather/realtime", weatherCtrl.Realtime)
		weatherGroup.GET("/:id/weather/minutely", weatherCtrl.Minutely)
		weatherGroup.GET("/:id/weather/hourly", weatherCtrl.Hourly)
		weatherGroup.GET("/:id/weather/daily", weatherCtrl.Daily)
		weatherGroup.GET("/:id/weather/alerts", weatherCtrl.Alerts)
		weatherGroup.GET("/:id/weather/life-indices", weatherCtrl.LifeIndices)
		weatherGroup.GET("/:id/weather/fetch-runs", weatherCtrl.FetchRuns)
	}
}

func registerMallWeatherRefreshRoutes(api *gin.RouterGroup, refreshCtrl *data_ctrl.MallWeatherRefreshController) {
	weatherGroup := api.Group("/v1/malls")
	weatherGroup.Use(middleware.AuthJWT())
	weatherGroup.POST("/:id/weather-refresh", refreshCtrl.Refresh)
}

func registerMallWeatherExportProfileRoutes(
	api *gin.RouterGroup,
	profileCtrl *data_ctrl.MallWeatherExportProfileController,
) {
	profileGroup := api.Group("/v1/weather-export-profiles")
	profileGroup.Use(middleware.AuthJWT())
	profileGroup.POST("", profileCtrl.Save)
	profileGroup.GET("", profileCtrl.List)
}

func registerMallWeatherExportJobRoutes(
	api *gin.RouterGroup,
	exportCtrl *data_ctrl.MallWeatherExportJobController,
) {
	exportGroup := api.Group("/v1/weather-exports")
	exportGroup.Use(middleware.AuthJWT())
	exportGroup.POST("", exportCtrl.Create)
	exportGroup.GET("/:job_id", exportCtrl.Get)
	exportGroup.GET("/:job_id/download", exportCtrl.Download)
}

func registerMallWeatherFeishuPushRoutes(
	api *gin.RouterGroup,
	pushCtrl *data_ctrl.MallWeatherFeishuPushController,
) {
	pushGroup := api.Group("/v1/weather-sheet-pushes")
	pushGroup.Use(middleware.AuthJWT())
	pushGroup.POST("", pushCtrl.Create)
	pushGroup.GET("/:run_id", pushCtrl.Get)
	pushGroup.POST("/dry-run", pushCtrl.DryRun)
}

func registerMallWeatherMetricsRoutes(
	api *gin.RouterGroup,
	metricsCtrl *data_ctrl.MallWeatherMetricsController,
) {
	metricsGroup := api.Group("/v1/mall-weather")
	metricsGroup.Use(middleware.AuthJWT())
	metricsGroup.GET("/metrics", metricsCtrl.Snapshot)
}

func registerMallRoutes(api *gin.RouterGroup, mallCtrl *data_ctrl.MallController) {
	mallGroup := api.Group("/v1/malls")
	mallGroup.Use(middleware.AuthJWT())
	{
		mallGroup.POST("", mallCtrl.Create)
		mallGroup.POST("/import", mallCtrl.Import)
		mallGroup.GET("", mallCtrl.List)
		mallGroup.GET("/:id", mallCtrl.Get)
		mallGroup.PATCH("/:id", mallCtrl.Update)
		mallGroup.DELETE("/:id", mallCtrl.Delete)
		mallGroup.POST("/:id/geocode", mallCtrl.TriggerGeocode)
		mallGroup.GET("/:id/geocode-candidates", mallCtrl.ListGeocodeCandidates)
		mallGroup.POST("/:id/geocode-confirm", mallCtrl.ConfirmGeocode)
	}
}
