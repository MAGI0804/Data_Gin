package job

import (
	"context"
	"encoding/json"

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

	// 这里只负责开启任务，具体的业务逻辑由服务层处理
	// 实际的处理逻辑会在服务层实现

	logger.Info("数据处理任务完成", zap.Uint("raw_data_id", payload.RawDataID))
	return nil
}
