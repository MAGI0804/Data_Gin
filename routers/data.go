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
		sourceGroup.POST("", sourceCtrl.CreateSource)
		sourceGroup.POST("/:id/test", sourceCtrl.TestSource)
		sourceGroup.POST("/:id/fetch", sourceCtrl.FetchSource)
	}

	transformGroup := api.Group("/v1/transform-rules")
	transformGroup.Use(middleware.AuthJWT())
	{
		transformCtrl := data_ctrl.NewTransformController()
		transformGroup.POST("", transformCtrl.CreateRule)
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
		destinationGroup.POST("", deliveryCtrl.CreateDestination)
		destinationGroup.POST("/:id/test", deliveryCtrl.TestDestination)
	}

	deliveryTaskGroup := api.Group("/v1/delivery-tasks")
	deliveryTaskGroup.Use(middleware.AuthJWT())
	{
		deliveryCtrl := data_ctrl.NewDeliveryController()
		deliveryTaskGroup.POST("", deliveryCtrl.CreateTask)
		deliveryTaskGroup.POST("/:id/run", deliveryCtrl.RunTask)
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
