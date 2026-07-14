package config_svc

import (
	"context"
	"encoding/json"
	"fmt"

	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/pkg/orderpush"
)

const orderPushSkipConfigKey = "order_push_skip_policy"

type RuntimeConfigStore interface {
	GetValue(ctx context.Context, key string) (string, bool, error)
	SetValue(ctx context.Context, key string, value string) error
}

type OrderPushSkipConfigService struct {
	configDAO RuntimeConfigStore
}

func NewOrderPushSkipConfigService() *OrderPushSkipConfigService {
	return &OrderPushSkipConfigService{
		configDAO: data_dao.NewRuntimeConfigDAO(),
	}
}

func (s *OrderPushSkipConfigService) Get(ctx context.Context) (orderpush.SkipPolicy, error) {
	value, exists, err := s.configDAO.GetValue(ctx, orderPushSkipConfigKey)
	if err != nil {
		return orderpush.SkipPolicy{}, err
	}
	if !exists {
		return orderpush.SkipPolicy{}, nil
	}
	return orderpush.ParseSkipPolicyJSON(value)
}

func (s *OrderPushSkipConfigService) Save(ctx context.Context, policy orderpush.SkipPolicy) (orderpush.SkipPolicy, error) {
	normalized, err := policy.Normalized()
	if err != nil {
		return orderpush.SkipPolicy{}, err
	}
	data, err := json.Marshal(normalized)
	if err != nil {
		return orderpush.SkipPolicy{}, fmt.Errorf("marshal order push skip config: %w", err)
	}
	if err := s.configDAO.SetValue(ctx, orderPushSkipConfigKey, string(data)); err != nil {
		return orderpush.SkipPolicy{}, err
	}
	return normalized, nil
}
