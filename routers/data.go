package routers

import (
	"gin-biz-web-api/internal/controller/data_ctrl"
	"gin-biz-web-api/internal/middleware"

	"github.com/gin-gonic/gin"
)

func apiData(api *gin.RouterGroup) {
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
	}

	runGroup := api.Group("/v1/runs")
	runGroup.Use(middleware.AuthJWT())
	{
		runCtrl := data_ctrl.NewRunController()
		runGroup.GET("", runCtrl.ListRuns)
	}

	excelMatchJobGroup := api.Group("/v1/excel-match-jobs")
	excelMatchJobGroup.Use(middleware.AuthJWT())
	{
		excelMatchJobCtrl := data_ctrl.NewExcelMatchJobController()
		excelMatchJobGroup.POST("", excelMatchJobCtrl.CreateJob)
		excelMatchJobGroup.GET("/:id", excelMatchJobCtrl.GetJob)
		excelMatchJobGroup.GET("/:id/download", excelMatchJobCtrl.Download)
	}

	legacyTaskGroup := api.Group("/v1/legacy-tasks")
	legacyTaskGroup.Use(middleware.AuthJWT())
	{
		legacyTaskCtrl := data_ctrl.NewLegacyTaskController()
		legacyTaskGroup.GET("", legacyTaskCtrl.List)
		legacyTaskGroup.POST("/:code/run", legacyTaskCtrl.Run)
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
