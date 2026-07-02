package data_svc

import (
	"context"
	"encoding/json"
	"fmt"

	transformconnector "gin-biz-web-api/connector/transform"
	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/model"
)

type TransformService struct {
	ruleDAO        *data_dao.TransformRuleDAO
	rawRecordDAO   *data_dao.RawRecordDAO
	cleanRecordDAO *data_dao.CleanRecordDAO
	pipelineRunDAO *data_dao.PipelineRunDAO
}

func NewTransformService() *TransformService {
	return &TransformService{
		ruleDAO:        data_dao.NewTransformRuleDAO(),
		rawRecordDAO:   data_dao.NewRawRecordDAO(),
		cleanRecordDAO: data_dao.NewCleanRecordDAO(),
		pipelineRunDAO: data_dao.NewPipelineRunDAO(),
	}
}

func (s *TransformService) CreateTransformRule(ctx context.Context, req *requestbody.TransformRuleCreateRequest) (*model.TransformRule, error) {
	if !json.Valid([]byte(req.ConfigJSON)) {
		return nil, fmt.Errorf("config_json must be valid json")
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	rule := &model.TransformRule{
		SourceID:   req.SourceID,
		Name:       req.Name,
		RuleType:   req.RuleType,
		OrderIndex: req.OrderIndex,
		ConfigJSON: req.ConfigJSON,
		Enabled:    enabled,
	}

	_, err := s.ruleDAO.Create(ctx, rule)
	if err != nil {
		return nil, err
	}

	return rule, nil
}

func (s *TransformService) ListTransformRules(ctx context.Context) ([]model.TransformRule, error) {
	return s.ruleDAO.FindAll(ctx)
}

func (s *TransformService) GetTransformRule(ctx context.Context, id uint) (*model.TransformRule, error) {
	return s.ruleDAO.FindByID(ctx, id)
}

func (s *TransformService) UpdateTransformRule(ctx context.Context, id uint, req *requestbody.TransformRuleUpdateRequest) (*model.TransformRule, error) {
	if !json.Valid([]byte(req.ConfigJSON)) {
		return nil, fmt.Errorf("config_json must be valid json")
	}

	rule, err := s.ruleDAO.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	rule.SourceID = req.SourceID
	rule.Name = req.Name
	rule.RuleType = req.RuleType
	rule.OrderIndex = req.OrderIndex
	rule.ConfigJSON = req.ConfigJSON
	rule.Enabled = enabled

	if err := s.ruleDAO.Update(ctx, rule); err != nil {
		return nil, err
	}
	return rule, nil
}

func (s *TransformService) TestMappingRule(ctx context.Context, req *requestbody.TransformRuleTestRequest) (map[string]interface{}, error) {
	_ = ctx

	cfg, err := decodeMappingConfig(req.ConfigJSON)
	if err != nil {
		return nil, err
	}

	return transformconnector.ApplyMapping(req.RawContent, cfg)
}

type TransformRawRecordResult struct {
	TraceID       string                 `json:"trace_id"`
	CleanRecordID uint                   `json:"clean_record_id"`
	CleanContent  map[string]interface{} `json:"clean_content"`
}

func (s *TransformService) TransformRawRecord(ctx context.Context, rawRecordID uint) (*TransformRawRecordResult, error) {
	rawRecord, err := s.rawRecordDAO.FindByID(ctx, rawRecordID)
	if err != nil {
		return nil, err
	}

	var rawContent map[string]interface{}
	if err := json.Unmarshal([]byte(rawRecord.RawContent), &rawContent); err != nil {
		s.rawRecordDAO.UpdateStatus(ctx, rawRecord.ID, "failed", err.Error())
		return nil, fmt.Errorf("decode raw_content: %w", err)
	}

	rules, err := s.ruleDAO.FindEnabledBySourceID(ctx, rawRecord.SourceID)
	if err != nil {
		return nil, err
	}
	if len(rules) == 0 {
		return nil, fmt.Errorf("no enabled transform rules for source_id %d", rawRecord.SourceID)
	}

	traceID := defaultString(rawRecord.TraceID, newTraceID())
	runID, err := s.pipelineRunDAO.Create(ctx, &model.PipelineRun{
		TraceID:      traceID,
		RunType:      "transform",
		TriggerType:  "manual",
		SourceID:     rawRecord.SourceID,
		Status:       "running",
		TotalCount:   1,
		SuccessCount: 0,
		FailedCount:  0,
	})
	if err != nil {
		return nil, err
	}

	cleanContent := map[string]interface{}{}
	tableName := defaultString(rawRecord.SourceCode, "clean_records")
	businessKeyField := ""

	for _, rule := range rules {
		if rule.RuleType != "mapping" {
			continue
		}

		cfg, err := decodeMappingConfig(rule.ConfigJSON)
		if err != nil {
			s.finishTransformRun(ctx, runID, "failed", err.Error())
			s.rawRecordDAO.UpdateStatus(ctx, rawRecord.ID, "failed", err.Error())
			return nil, err
		}

		mapped, err := transformconnector.ApplyMapping(rawContent, cfg)
		if err != nil {
			s.finishTransformRun(ctx, runID, "failed", err.Error())
			s.rawRecordDAO.UpdateStatus(ctx, rawRecord.ID, "failed", err.Error())
			return nil, err
		}

		for key, value := range mapped {
			cleanContent[key] = value
		}
		if cfg.TableName != "" {
			tableName = cfg.TableName
		}
		if cfg.BusinessKeyField != "" {
			businessKeyField = cfg.BusinessKeyField
		}
	}

	cleanJSON, err := json.Marshal(cleanContent)
	if err != nil {
		s.finishTransformRun(ctx, runID, "failed", err.Error())
		return nil, err
	}

	cleanRecord := &model.CleanRecord{
		RawRecordID:      rawRecord.ID,
		SourceID:         rawRecord.SourceID,
		LogicalTableName: tableName,
		BusinessKey:      businessKey(cleanContent, businessKeyField),
		CleanContent:     string(cleanJSON),
		QualityScore:     100,
		Status:           "ready",
	}
	cleanRecordID, err := s.cleanRecordDAO.Create(ctx, cleanRecord)
	if err != nil {
		s.finishTransformRun(ctx, runID, "failed", err.Error())
		s.rawRecordDAO.UpdateStatus(ctx, rawRecord.ID, "failed", err.Error())
		return nil, err
	}

	s.rawRecordDAO.UpdateStatus(ctx, rawRecord.ID, "cleaned", "")
	s.pipelineRunDAO.Finish(ctx, runID, "success", 1, 0, "")

	return &TransformRawRecordResult{
		TraceID:       traceID,
		CleanRecordID: cleanRecordID,
		CleanContent:  cleanContent,
	}, nil
}

func decodeMappingConfig(configJSON string) (transformconnector.MappingConfig, error) {
	var cfg transformconnector.MappingConfig
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return cfg, fmt.Errorf("decode mapping config: %w", err)
	}
	return cfg, nil
}

func (s *TransformService) finishTransformRun(ctx context.Context, runID uint, status, errMessage string) {
	if runID == 0 {
		return
	}
	s.pipelineRunDAO.Finish(ctx, runID, status, 0, 1, errMessage)
}

func businessKey(cleanContent map[string]interface{}, field string) string {
	if field == "" {
		return ""
	}
	value, ok := cleanContent[field]
	if !ok || value == nil {
		return ""
	}
	return fmt.Sprintf("%v", value)
}
