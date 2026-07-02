package data_svc

import (
	"context"
	"encoding/json"
	"strings"

	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/model"
)

type SourceService struct {
	sourceDAO *data_dao.SourceDefinitionDAO
}

func NewSourceService() *SourceService {
	return &SourceService{
		sourceDAO: data_dao.NewSourceDefinitionDAO(),
	}
}

func (s *SourceService) CreateSourceDefinition(ctx context.Context, req *requestbody.SourceDefinitionCreateRequest) (*model.SourceDefinition, error) {
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
		ConfigJSON:     defaultJSON(req.ConfigJSON, "{}"),
		SchemaJSON:     defaultJSON(req.SchemaJSON, "{}"),
		DedupeKeys:     defaultJSON(req.DedupeKeys, "[]"),
		SourceQueryKey: strings.TrimSpace(req.SourceQueryKey),
	}

	_, err := s.sourceDAO.Create(ctx, source)
	if err != nil {
		return nil, err
	}

	return source, nil
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
