package job

import (
	"context"
	"encoding/json"

	"gin-biz-web-api/Trigger"
	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/pkg/logger"

	"github.com/hibiken/asynq"
	"go.uber.org/zap"
)

const TypeDataProcess = "data:process"

// DataProcessPayload 数据处理任务 payload
type DataProcessPayload struct {
	RawDataID uint `json:"raw_data_id"`
}

// NewDataProcessTask 创建数据处理任务
func NewDataProcessTask(rawDataID uint) *asynq.Task {
	payload, _ := json.Marshal(DataProcessPayload{
		RawDataID: rawDataID,
	})

	return asynq.NewTask(
		TypeDataProcess,
		payload,
		asynq.Queue(DefaultQueueName),
		asynq.MaxRetry(3),
	)
}

// HandleDataProcessTask 处理数据处理任务
func HandleDataProcessTask(ctx context.Context, t *asynq.Task) error {
	var payload DataProcessPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return err
	}

	logger.Info("开始处理数据任务", zap.Uint("raw_data_id", payload.RawDataID))

	// 1. 获取原始数据
	rawDataDAO := data_dao.NewRawDataDAO()
	rawData, err := rawDataDAO.FindByID(ctx, payload.RawDataID)
	if err != nil {
		logger.Error("获取原始数据失败", zap.Uint("raw_data_id", payload.RawDataID), zap.Error(err))
		return err
	}

	// 2. 解析元数据，检查是否为企迈订单
	var metadata map[string]interface{}
	if err := json.Unmarshal([]byte(rawData.Metadata), &metadata); err != nil {
		logger.Error("解析元数据失败", zap.Uint("raw_data_id", payload.RawDataID), zap.Error(err))
		return err
	}

	// 3. 调用企迈订单触发器
	qimaiTrigger := Trigger.NewQimaiOrderTrigger()
	if err := qimaiTrigger.Trigger(ctx, rawData); err != nil {
		logger.Error("企迈订单触发器执行失败", zap.Uint("raw_data_id", payload.RawDataID), zap.Error(err))
		// 不返回错误，允许任务继续执行
	}

	logger.Info("数据处理任务完成", zap.Uint("raw_data_id", payload.RawDataID))
	return nil
}
