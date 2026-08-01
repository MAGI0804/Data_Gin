package data_svc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	sourceconnector "gin-biz-web-api/connector/source"
	"gin-biz-web-api/internal/configsecret"
	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/job"
	"gin-biz-web-api/model"
	jobClient "gin-biz-web-api/pkg/job"
)

type SourceService struct {
	sourceDAO      *data_dao.SourceDefinitionDAO
	rawRecordDAO   *data_dao.RawRecordDAO
	rawDataDAO     *data_dao.RawDataDAO
	pipelineRunDAO *data_dao.PipelineRunDAO
	connectors     map[string]sourceconnector.Connector
}

func NewSourceService() *SourceService {
	return &SourceService{
		sourceDAO:      data_dao.NewSourceDefinitionDAO(),
		rawRecordDAO:   data_dao.NewRawRecordDAO(),
		rawDataDAO:     data_dao.NewRawDataDAO(),
		pipelineRunDAO: data_dao.NewPipelineRunDAO(),
		connectors:     sourceconnector.Builtins(),
	}
}

func (s *SourceService) CreateSourceDefinition(ctx context.Context, req *requestbody.SourceDefinitionCreateRequest) (*model.SourceDefinition, error) {
	configJSON, err := configsecret.NewJSON(req.ConfigJSON, "{}")
	if err != nil {
		return nil, err
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	source := &model.SourceDefinition{
		Name:           strings.TrimSpace(req.Name),
		Code:           strings.TrimSpace(req.Code),
		SourceType:     strings.TrimSpace(req.SourceType),
		Enabled:        enabled,
		AuthType:       defaultString(strings.TrimSpace(req.AuthType), "none"),
		ConfigJSON:     configJSON,
		SchemaJSON:     defaultJSON(req.SchemaJSON, "{}"),
		DedupeKeys:     defaultJSON(req.DedupeKeys, "[]"),
		SourceQueryKey: strings.TrimSpace(req.SourceQueryKey),
	}

	_, err = s.sourceDAO.Create(ctx, source)
	if err != nil {
		return nil, err
	}

	return source, nil
}

func (s *SourceService) ListSourceDefinitions(ctx context.Context) ([]model.SourceDefinition, error) {
	return s.sourceDAO.FindAll(ctx)
}

func (s *SourceService) ListSourceDefinitionsPage(ctx context.Context, query data_dao.SourceDefinitionListQuery) (*data_dao.SourceDefinitionListPage, error) {
	return s.sourceDAO.FindPage(ctx, query)
}

func (s *SourceService) GetSourceDefinition(ctx context.Context, id uint) (*model.SourceDefinition, error) {
	return s.sourceDAO.FindByID(ctx, id)
}

func (s *SourceService) UpdateSourceDefinition(ctx context.Context, id uint, req *requestbody.SourceDefinitionUpdateRequest) (*model.SourceDefinition, error) {
	source, err := s.sourceDAO.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	configJSON, err := configsecret.MergeJSON(source.ConfigJSON, req.ConfigJSON)
	if err != nil {
		return nil, err
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	source.Name = strings.TrimSpace(req.Name)
	source.Code = strings.TrimSpace(req.Code)
	source.SourceType = strings.TrimSpace(req.SourceType)
	source.Enabled = enabled
	source.AuthType = defaultString(strings.TrimSpace(req.AuthType), "none")
	source.ConfigJSON = configJSON
	source.SchemaJSON = defaultJSON(req.SchemaJSON, "{}")
	source.DedupeKeys = defaultJSON(req.DedupeKeys, "[]")
	source.SourceQueryKey = strings.TrimSpace(req.SourceQueryKey)

	if err := s.sourceDAO.Update(ctx, source); err != nil {
		return nil, err
	}
	return source, nil
}

func (s *SourceService) TestSourceDefinition(ctx context.Context, id uint) error {
	source, err := s.sourceDAO.FindByID(ctx, id)
	if err != nil {
		return err
	}

	connector, cfg, err := s.connectorForSource(source)
	if err != nil {
		return err
	}

	return connector.Test(ctx, cfg)
}

type SourceFetchResult struct {
	TraceID      string `json:"trace_id"`
	TotalCount   int    `json:"total_count"`
	SuccessCount int    `json:"success_count"`
	FailedCount  int    `json:"failed_count"`
}

func (s *SourceService) FetchSourceDefinition(ctx context.Context, id uint) (*SourceFetchResult, error) {
	source, err := s.sourceDAO.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !source.Enabled {
		return nil, fmt.Errorf("source %q is disabled", source.Code)
	}

	connector, cfg, err := s.connectorForSource(source)
	if err != nil {
		return nil, err
	}

	traceID := newTraceID()
	now := time.Now()
	runID, err := s.pipelineRunDAO.Create(ctx, &model.PipelineRun{
		TraceID:      traceID,
		RunType:      "fetch",
		TriggerType:  "manual",
		SourceID:     source.ID,
		Status:       "running",
		StartedAt:    &model.TimeNormal{Time: now},
		SuccessCount: 0,
		FailedCount:  0,
	})
	if err != nil {
		return nil, err
	}

	fetchResult, err := connector.Fetch(ctx, cfg, sourceconnector.FetchCursor{})
	if err != nil {
		s.pipelineRunDAO.Finish(ctx, runID, "failed", 0, 1, err.Error())
		return nil, err
	}

	successCount := 0
	failedCount := 0
	for _, record := range fetchResult.Records {
		if err := s.saveFetchedRecord(ctx, source, traceID, record); err != nil {
			failedCount++
			continue
		}
		successCount++
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

	return &SourceFetchResult{
		TraceID:      traceID,
		TotalCount:   len(fetchResult.Records),
		SuccessCount: successCount,
		FailedCount:  failedCount,
	}, nil
}

func (s *SourceService) connectorForSource(source *model.SourceDefinition) (sourceconnector.Connector, sourceconnector.Config, error) {
	connectorType := source.SourceType
	if connectorType == "api" {
		connectorType = "api_poll"
	}

	connector, ok := s.connectors[connectorType]
	if !ok {
		return nil, nil, fmt.Errorf("unsupported source connector %q", source.SourceType)
	}

	cfg := sourceconnector.Config{}
	if source.ConfigJSON != "" {
		if err := json.Unmarshal([]byte(source.ConfigJSON), &cfg); err != nil {
			return nil, nil, fmt.Errorf("decode source config_json: %w", err)
		}
	}

	return connector, cfg, nil
}

func (s *SourceService) saveFetchedRecord(
	ctx context.Context,
	source *model.SourceDefinition,
	traceID string,
	record map[string]interface{},
) error {
	rawContentJSON, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("encode fetched record: %w", err)
	}

	receivedAt := time.Now()
	metadata := map[string]interface{}{
		"trace_id":    traceID,
		"source":      source.Code,
		"source_type": source.SourceType,
		"format":      "fetch",
		"ingested_at": receivedAt.Unix(),
	}
	metadataJSON, _ := json.Marshal(metadata)

	rawRecord := &model.RawRecord{
		SourceID:     source.ID,
		SourceCode:   source.Code,
		RawContent:   string(rawContentJSON),
		MetadataJSON: string(metadataJSON),
		Status:       "received",
		TraceID:      traceID,
		ReceivedAt:   &model.TimeNormal{Time: receivedAt},
	}
	rawRecordID, err := s.rawRecordDAO.Create(ctx, rawRecord)
	if err != nil {
		return fmt.Errorf("save raw_record: %w", err)
	}

	rawData := &model.RawData{
		DataSourceID: source.ID,
		DataType:     defaultString(source.SourceType, "raw"),
		RawContent:   string(rawContentJSON),
		Metadata:     string(metadataJSON),
		Status:       "pending",
		Source:       source.Code,
		IngestedAt:   &receivedAt,
	}

	rawDataID, err := s.rawDataDAO.Create(ctx, rawData)
	if err != nil {
		s.rawRecordDAO.UpdateStatus(ctx, rawRecordID, "failed", err.Error())
		return fmt.Errorf("save legacy raw_data: %w", err)
	}

	task := job.NewDataProcessTask(rawDataID)
	if _, err := jobClient.Client.Enqueue(task); err != nil {
		s.rawRecordDAO.UpdateStatus(ctx, rawRecordID, "failed", err.Error())
		return fmt.Errorf("enqueue data process task: %w", err)
	}

	return s.rawRecordDAO.UpdateStatus(ctx, rawRecordID, "queued", "")
}

func newTraceID() string {
	return strings.ReplaceAll(time.Now().Format("20060102150405.000000000"), ".", "")
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func defaultJSON(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	if !json.Valid([]byte(value)) {
		return fallback
	}
	return value
}
