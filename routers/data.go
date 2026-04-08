package routers

import (
	"gin-biz-web-api/internal/controller/data_ctrl"
	"gin-biz-web-api/internal/middleware"

	"github.com/gin-gonic/gin"
)

func apiData(api *gin.RouterGroup) {
	dataGroup := api.Group("/v1/data")
	dataGroup.Use(middleware.AuthJWT()) // 需要认证
	{
		dataCtrl := data_ctrl.NewDataController()

		// 数据采集
		dataGroup.POST("/collect/:source_id", dataCtrl.CollectController.ManualCollect)
		dataGroup.GET("/collect/status/:job_id", dataCtrl.CollectController.CollectStatus)

		// 数据接收
		dataGroup.POST("/ingest", dataCtrl.IngestController.IngestData)
		dataGroup.POST("/ingest/batch", dataCtrl.IngestController.IngestBatchData)
		dataGroup.POST("/ingest/raw", dataCtrl.IngestController.RawIngestData) // 接收原始格式数据

		// 数据查询
		dataGroup.GET("/raw", dataCtrl.QueryController.GetRawData)
		dataGroup.GET("/processed", dataCtrl.QueryController.GetProcessedData)
		dataGroup.GET("/statistics", dataCtrl.QueryController.GetStatistics)
	}
}
