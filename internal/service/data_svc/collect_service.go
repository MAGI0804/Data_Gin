package data_svc

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/job"
	"gin-biz-web-api/model"
	jobClient "gin-biz-web-api/pkg/job"
	"gin-biz-web-api/pkg/logger"

	"go.uber.org/zap"
)

type CollectService struct {
	dataSourceDAO *data_dao.DataSourceDAO
	rawDataDAO    *data_dao.RawDataDAO
}

func NewCollectService() *CollectService {
	return &CollectService{
		dataSourceDAO: data_dao.NewDataSourceDAO(),
		rawDataDAO:    data_dao.NewRawDataDAO(),
	}
}

// CollectFromSource 从指定数据源采集数据
func (s *CollectService) CollectFromSource(ctx context.Context, sourceID uint) (string, error) {
	// 1. 获取数据源配置
	source, err := s.dataSourceDAO.FindByID(ctx, sourceID)
	if err != nil {
		logger.Error("获取数据源失败", zap.Uint("source_id", sourceID), zap.Error(err))
		return "", fmt.Errorf("获取数据源失败: %w", err)
	}

	// 2. 根据类型调用不同的采集器
	var data []map[string]interface{}
	switch source.Type {
	case "api":
		data, err = s.collectFromAPI(ctx, source)
	case "database":
		data, err = s.collectFromDatabase(ctx, source)
	case "file":
		data, err = s.collectFromFile(ctx, source)
	default:
		err = fmt.Errorf("不支持的数据源类型: %s", source.Type)
	}

	if err != nil {
		// 更新数据源状态为错误
		updateErr := s.dataSourceDAO.UpdateStatus(ctx, sourceID, "error", err.Error())
		if updateErr != nil {
			logger.Error("更新数据源状态失败", zap.Error(updateErr))
		}
		return "", fmt.Errorf("数据采集失败: %w", err)
	}

	// 3. 保存原始数据
	var rawDataIDs []uint
	for _, item := range data {
		ingestedTime := time.Now()
		metadata := map[string]interface{}{
			"ingested_at": ingestedTime.Unix(),
			"source":      source.Name,
		}
		metadataJSON, _ := json.Marshal(metadata)

		rawData := &model.RawData{
			DataSourceID: sourceID,
			DataType:     source.Type,
			RawContent:   fmt.Sprintf("%v", item),
			Metadata:     string(metadataJSON),
			Status:       "pending",
			Source:       source.Name,
			IngestedAt:   &ingestedTime,
		}

		id, err := s.rawDataDAO.Create(ctx, rawData)
		if err != nil {
			logger.Error("保存原始数据失败", zap.Error(err))
			continue
		}
		rawDataIDs = append(rawDataIDs, id)

		// 4. 投递异步处理任务
		task := job.NewDataProcessTask(id)

		_, err = jobClient.Client.Enqueue(task)
		if err != nil {
			logger.Error("投递任务失败", zap.Error(err))
			continue
		}
	}

	// 5. 更新数据源状态
	s.dataSourceDAO.UpdateSyncStatus(ctx, sourceID, time.Now(), "success")

	return fmt.Sprintf("采集 %d 条数据，保存 %d 条数据", len(data), len(rawDataIDs)), nil
}

// collectFromAPI 从API采集数据
func (s *CollectService) collectFromAPI(ctx context.Context, source *model.DataSource) ([]map[string]interface{}, error) {
	// 实现API调用逻辑
	// 这里简化示例，实际需要根据配置调用HTTP API
	logger.Info("从API采集数据", zap.String("source_name", source.Name))

	// 模拟API返回数据
	return []map[string]interface{}{
		{"id": 1, "name": "测试数据1", "value": 100},
		{"id": 2, "name": "测试数据2", "value": 200},
	}, nil
}

// collectFromDatabase 从数据库采集数据
func (s *CollectService) collectFromDatabase(ctx context.Context, source *model.DataSource) ([]map[string]interface{}, error) {
	// 实现数据库采集逻辑
	logger.Info("从数据库采集数据", zap.String("source_name", source.Name))

	// 模拟数据库返回数据
	return []map[string]interface{}{
		{"id": 3, "name": "数据库数据1", "value": 300},
		{"id": 4, "name": "数据库数据2", "value": 400},
	}, nil
}

// collectFromFile 从文件采集数据
func (s *CollectService) collectFromFile(ctx context.Context, source *model.DataSource) ([]map[string]interface{}, error) {
	// 实现文件采集逻辑
	logger.Info("从文件采集数据", zap.String("source_name", source.Name))

	// 模拟文件返回数据
	return []map[string]interface{}{
		{"id": 5, "name": "文件数据1", "value": 500},
		{"id": 6, "name": "文件数据2", "value": 600},
	}, nil
}
