package data_ctrl

// DataController 数据控制器主结构
type DataController struct {
	CollectController *CollectController
	IngestController  *IngestController
	QueryController   *QueryController
}

// NewDataController 创建数据控制器实例
func NewDataController() *DataController {
	return &DataController{
		CollectController: NewCollectController(),
		IngestController:  NewIngestController(),
		QueryController:   NewQueryController(),
	}
}
