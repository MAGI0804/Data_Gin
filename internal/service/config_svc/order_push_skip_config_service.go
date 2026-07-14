package config_svc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/orderpush"
)

const orderPushSkipConfigKey = "order_push_skip_policy"

type RuntimeConfigStore interface {
	GetValue(ctx context.Context, key string) (string, bool, error)
	SetValue(ctx context.Context, key string, value string) error
}

type pushDestinationLister interface {
	FindAll(ctx context.Context) ([]model.DestinationDefinition, error)
}

type OrderPushSkipConfigService struct {
	configDAO      RuntimeConfigStore
	destinationDAO pushDestinationLister
}

func NewOrderPushSkipConfigService() *OrderPushSkipConfigService {
	return &OrderPushSkipConfigService{
		configDAO:      data_dao.NewRuntimeConfigDAO(),
		destinationDAO: data_dao.NewDestinationDefinitionDAO(),
	}
}

func (s *OrderPushSkipConfigService) Get(ctx context.Context) (orderpush.SkipConfig, error) {
	value, exists, err := s.configDAO.GetValue(ctx, orderPushSkipConfigKey)
	if err != nil {
		return orderpush.SkipConfig{}, err
	}
	if !exists {
		return orderpush.SkipConfig{}, nil
	}
	return orderpush.ParseSkipConfigJSON(value)
}

func (s *OrderPushSkipConfigService) Save(ctx context.Context, config orderpush.SkipConfig) (orderpush.SkipConfig, error) {
	normalized, err := config.Normalized()
	if err != nil {
		return orderpush.SkipConfig{}, err
	}
	data, err := json.Marshal(normalized)
	if err != nil {
		return orderpush.SkipConfig{}, fmt.Errorf("marshal order push skip config: %w", err)
	}
	if err := s.configDAO.SetValue(ctx, orderPushSkipConfigKey, string(data)); err != nil {
		return orderpush.SkipConfig{}, err
	}
	return normalized, nil
}

func (s *OrderPushSkipConfigService) ListTargets(ctx context.Context) ([]orderpush.TargetOption, error) {
	targets := builtinOrderPushTargets()
	if s.destinationDAO == nil {
		return targets, nil
	}
	destinations, err := s.destinationDAO.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	for _, destination := range destinations {
		targets = appendOrderPushTarget(targets, destination.Code, destination.Name)
	}
	return targets, nil
}

func builtinOrderPushTargets() []orderpush.TargetOption {
	targets := []orderpush.TargetOption{}
	targets = appendOrderPushTarget(targets, "hangzhou_henglong", "杭州恒隆")
	targets = appendOrderPushTarget(targets, "jialicheng", "嘉里城")
	targets = appendOrderPushTarget(targets, "panlong", "蟠龙")
	targets = appendOrderPushTarget(targets, "qiantan", "前滩")
	targets = appendOrderPushTarget(targets, "shangsheng", "上生新所")
	targets = appendOrderPushTarget(targets, "xintiandi", "新天地")
	return targets
}

func appendOrderPushTarget(targets []orderpush.TargetOption, code, name string) []orderpush.TargetOption {
	code = strings.TrimSpace(code)
	name = strings.TrimSpace(name)
	if code == "" {
		return targets
	}
	for _, target := range targets {
		if strings.EqualFold(target.Code, code) {
			return targets
		}
	}
	if name == "" {
		name = code
	}
	return append(targets, orderpush.TargetOption{Code: code, Name: name})
}
