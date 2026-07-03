package data_svc

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	destinationconnector "gin-biz-web-api/connector/destination"
	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/app"
)

type DeliveryService struct {
	destinationDAO *data_dao.DestinationDefinitionDAO
	taskDAO        *data_dao.DeliveryTaskDAO
	cleanDAO       *data_dao.CleanRecordDAO
	logDAO         *data_dao.DeliveryLogDAO
	pipelineRunDAO *data_dao.PipelineRunDAO
	publishers     map[string]destinationconnector.Publisher
}

type DeliveryLogRetryResult struct {
	LogID        uint   `json:"log_id"`
	SourceCode   string `json:"source_code"`
	BusinessKey  string `json:"business_key"`
	TargetCode   string `json:"target_code"`
	TargetName   string `json:"target_name"`
	Success      bool   `json:"success"`
	Skipped      bool   `json:"skipped"`
	ErrorMessage string `json:"error_message"`
}

func NewDeliveryService() *DeliveryService {
	return &DeliveryService{
		destinationDAO: data_dao.NewDestinationDefinitionDAO(),
		taskDAO:        data_dao.NewDeliveryTaskDAO(),
		cleanDAO:       data_dao.NewCleanRecordDAO(),
		logDAO:         data_dao.NewDeliveryLogDAO(),
		pipelineRunDAO: data_dao.NewPipelineRunDAO(),
		publishers:     destinationconnector.Builtins(),
	}
}

func (s *DeliveryService) CreateDestination(ctx context.Context, req *requestbody.DestinationCreateRequest) (*model.DestinationDefinition, error) {
	if !json.Valid([]byte(req.ConfigJSON)) {
		return nil, fmt.Errorf("config_json must be valid json")
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	destination := &model.DestinationDefinition{
		Name:            req.Name,
		Code:            req.Code,
		DestinationType: req.DestinationType,
		ConfigJSON:      req.ConfigJSON,
		Enabled:         enabled,
	}
	_, err := s.destinationDAO.Create(ctx, destination)
	if err != nil {
		return nil, err
	}
	return destination, nil
}

func (s *DeliveryService) ListDestinations(ctx context.Context) ([]model.DestinationDefinition, error) {
	return s.destinationDAO.FindAll(ctx)
}

func (s *DeliveryService) GetDestination(ctx context.Context, id uint) (*model.DestinationDefinition, error) {
	return s.destinationDAO.FindByID(ctx, id)
}

func (s *DeliveryService) UpdateDestination(ctx context.Context, id uint, req *requestbody.DestinationUpdateRequest) (*model.DestinationDefinition, error) {
	if !json.Valid([]byte(req.ConfigJSON)) {
		return nil, fmt.Errorf("config_json must be valid json")
	}

	destination, err := s.destinationDAO.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	destination.Name = req.Name
	destination.Code = req.Code
	destination.DestinationType = req.DestinationType
	destination.ConfigJSON = req.ConfigJSON
	destination.Enabled = enabled

	if err := s.destinationDAO.Update(ctx, destination); err != nil {
		return nil, err
	}
	return destination, nil
}

func (s *DeliveryService) TestDestination(ctx context.Context, id uint) error {
	destination, err := s.destinationDAO.FindByID(ctx, id)
	if err != nil {
		return err
	}

	publisher, cfg, err := s.publisherForDestination(destination)
	if err != nil {
		return err
	}
	return publisher.Test(ctx, cfg)
}

func (s *DeliveryService) CreateDeliveryTask(ctx context.Context, req *requestbody.DeliveryTaskCreateRequest) (*model.DeliveryTask, error) {
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	task := &model.DeliveryTask{
		Name:            req.Name,
		SourceID:        req.SourceID,
		CleanTable:      req.CleanTable,
		DestinationID:   req.DestinationID,
		TriggerType:     req.TriggerType,
		CronExpr:        req.CronExpr,
		FilterJSON:      defaultJSON(req.FilterJSON, "{}"),
		PayloadTemplate: req.PayloadTemplate,
		Enabled:         enabled,
	}
	_, err := s.taskDAO.Create(ctx, task)
	if err != nil {
		return nil, err
	}
	return task, nil
}

func (s *DeliveryService) ListDeliveryTasks(ctx context.Context) ([]model.DeliveryTask, error) {
	return s.taskDAO.FindAll(ctx)
}

func (s *DeliveryService) GetDeliveryTask(ctx context.Context, id uint) (*model.DeliveryTask, error) {
	return s.taskDAO.FindByID(ctx, id)
}

func (s *DeliveryService) UpdateDeliveryTask(ctx context.Context, id uint, req *requestbody.DeliveryTaskUpdateRequest) (*model.DeliveryTask, error) {
	task, err := s.taskDAO.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	task.Name = req.Name
	task.SourceID = req.SourceID
	task.CleanTable = req.CleanTable
	task.DestinationID = req.DestinationID
	task.TriggerType = req.TriggerType
	task.CronExpr = req.CronExpr
	task.FilterJSON = defaultJSON(req.FilterJSON, "{}")
	task.PayloadTemplate = req.PayloadTemplate
	task.Enabled = enabled

	if err := s.taskDAO.Update(ctx, task); err != nil {
		return nil, err
	}
	return task, nil
}

func (s *DeliveryService) ListDeliveryLogs(ctx context.Context, limit int) ([]model.DeliveryLog, error) {
	return s.logDAO.FindRecent(ctx, limit)
}

func (s *DeliveryService) RetryDeliveryLog(ctx context.Context, id uint) (*DeliveryLogRetryResult, error) {
	log, err := s.logDAO.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("find delivery log: %w", err)
	}
	if log.Success {
		return nil, fmt.Errorf("delivery log %d is already successful", id)
	}
	if log.SourceCode != bojunOrderPushSource {
		return nil, fmt.Errorf("delivery log %d source %q does not support retry", id, log.SourceCode)
	}
	if log.CleanRecordID == 0 {
		return nil, fmt.Errorf("delivery log %d missing clean record id", id)
	}

	order, err := data_dao.NewBojunRetailOrderDAO().FindByID(ctx, log.CleanRecordID)
	if err != nil {
		return nil, fmt.Errorf("find bojun retail order: %w", err)
	}
	if err := s.logDAO.IncrementRetryCount(ctx, id); err != nil {
		return nil, fmt.Errorf("increment retry count: %w", err)
	}

	pushResult := NewBojunOrderPushService().PushNewOrder(ctx, order)
	result := &DeliveryLogRetryResult{
		LogID:       id,
		SourceCode:  log.SourceCode,
		BusinessKey: log.BusinessKey,
		TargetCode:  pushResult.Target.Code,
		TargetName:  pushResult.Target.Name,
		Success:     pushResult.Success,
		Skipped:     pushResult.Skipped,
	}
	if pushResult.Error != nil {
		result.ErrorMessage = pushResult.Error.Error()
	}
	return result, nil
}

type DeliveryRunResult struct {
	TraceID      string `json:"trace_id"`
	TotalCount   int    `json:"total_count"`
	SuccessCount int    `json:"success_count"`
	FailedCount  int    `json:"failed_count"`
}

func (s *DeliveryService) RunDeliveryTask(ctx context.Context, taskID uint) (*DeliveryRunResult, error) {
	task, err := s.taskDAO.FindByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if !task.Enabled {
		return nil, fmt.Errorf("delivery task %d is disabled", task.ID)
	}

	destination, err := s.destinationDAO.FindByID(ctx, task.DestinationID)
	if err != nil {
		return nil, err
	}
	if !destination.Enabled {
		return nil, fmt.Errorf("destination %q is disabled", destination.Code)
	}

	publisher, cfg, err := s.publisherForDestination(destination)
	if err != nil {
		return nil, err
	}
	if task.PayloadTemplate != "" {
		cfg["payload_template"] = task.PayloadTemplate
	}

	cleanRecords, err := s.cleanDAO.FindReadyBySourceAndTable(ctx, task.SourceID, task.CleanTable, 100)
	if err != nil {
		return nil, err
	}

	traceID := newTraceID()
	runID, err := s.pipelineRunDAO.Create(ctx, &model.PipelineRun{
		TraceID:       traceID,
		RunType:       "delivery",
		TriggerType:   task.TriggerType,
		SourceID:      task.SourceID,
		DestinationID: task.DestinationID,
		Status:        "running",
		TotalCount:    len(cleanRecords),
		StartedAt:     &model.TimeNormal{Time: time.Now()},
	})
	if err != nil {
		return nil, err
	}

	successCount := 0
	failedCount := 0
	for _, cleanRecord := range cleanRecords {
		if s.publishCleanRecord(ctx, runID, traceID, destination, publisher, cfg, cleanRecord) {
			successCount++
			continue
		}
		failedCount++
	}

	status := "success"
	if failedCount > 0 && successCount > 0 {
		status = "partial_success"
	} else if failedCount > 0 {
		status = "failed"
	}
	if err := s.pipelineRunDAO.Finish(ctx, runID, status, successCount, failedCount, ""); err != nil {
		return nil, err
	}

	return &DeliveryRunResult{
		TraceID:      traceID,
		TotalCount:   len(cleanRecords),
		SuccessCount: successCount,
		FailedCount:  failedCount,
	}, nil
}

func (s *DeliveryService) publishCleanRecord(
	ctx context.Context,
	runID uint,
	traceID string,
	destination *model.DestinationDefinition,
	publisher destinationconnector.Publisher,
	cfg destinationconnector.Config,
	cleanRecord model.CleanRecord,
) bool {
	var content map[string]interface{}
	if err := json.Unmarshal([]byte(cleanRecord.CleanContent), &content); err != nil {
		s.createDeliveryLog(ctx, traceID, runID, destination, cleanRecord, nil, err)
		return false
	}

	result, err := publisher.Publish(ctx, cfg, destinationconnector.CleanRecord{
		ID:          cleanRecord.ID,
		BusinessKey: cleanRecord.BusinessKey,
		Content:     content,
	})
	s.createDeliveryLog(ctx, traceID, runID, destination, cleanRecord, result, err)
	if err != nil || result == nil || !result.Success {
		return false
	}

	return s.cleanDAO.MarkDelivered(ctx, cleanRecord.ID) == nil
}

func (s *DeliveryService) createDeliveryLog(
	ctx context.Context,
	traceID string,
	runID uint,
	destination *model.DestinationDefinition,
	cleanRecord model.CleanRecord,
	result *destinationconnector.PublishResult,
	publishErr error,
) {
	log := &model.DeliveryLog{
		TraceID:         traceID,
		RunID:           runID,
		CleanRecordID:   cleanRecord.ID,
		DestinationID:   destination.ID,
		SourceCode:      cleanRecord.LogicalTableName,
		DestinationCode: destination.Code,
		DestinationName: destination.Name,
		BusinessKey:     cleanRecord.BusinessKey,
		SentAt:          &model.TimeNormal{Time: app.TimeNowInTimezone()},
	}
	if result != nil {
		log.RequestBody = result.RequestBody
		log.ResponseBody = result.ResponseBody
		log.HTTPStatus = result.HTTPStatus
		log.Success = result.Success
		log.ErrorMessage = result.ErrorMessage
	}
	if publishErr != nil {
		log.Success = false
		log.ErrorMessage = publishErr.Error()
	}
	_, _ = s.logDAO.Create(ctx, log)
}

func (s *DeliveryService) publisherForDestination(destination *model.DestinationDefinition) (destinationconnector.Publisher, destinationconnector.Config, error) {
	publisher, ok := s.publishers[destination.DestinationType]
	if !ok {
		return nil, nil, fmt.Errorf("unsupported destination publisher %q", destination.DestinationType)
	}

	cfg := destinationconnector.Config{}
	if err := json.Unmarshal([]byte(destination.ConfigJSON), &cfg); err != nil {
		return nil, nil, fmt.Errorf("decode destination config_json: %w", err)
	}
	return publisher, cfg, nil
}
