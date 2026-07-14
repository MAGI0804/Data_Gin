package data_svc

import (
	"context"
	"encoding/json"
	"fmt"

	"gin-biz-web-api/internal/dao/data_dao"
)

type runtimeConfigStore interface {
	GetValue(ctx context.Context, key string) (string, bool, error)
	SetValue(ctx context.Context, key string, value string) error
}

type OrderPushSkipConfigService struct {
	configDAO runtimeConfigStore
}

func NewOrderPushSkipConfigService() *OrderPushSkipConfigService {
	return &OrderPushSkipConfigService{
		configDAO: data_dao.NewRuntimeConfigDAO(),
	}
}

func (s *OrderPushSkipConfigService) Get(ctx context.Context) (OrderPushSkipPolicy, error) {
	value, exists, err := s.configDAO.GetValue(ctx, orderPushSkipConfigKey)
	if err != nil {
		return OrderPushSkipPolicy{}, err
	}
	if !exists {
		return OrderPushSkipPolicy{}, nil
	}
	return parseOrderPushSkipPolicyJSON(value)
}

func (s *OrderPushSkipConfigService) Save(ctx context.Context, policy OrderPushSkipPolicy) (OrderPushSkipPolicy, error) {
	normalized, err := policy.Normalized()
	if err != nil {
		return OrderPushSkipPolicy{}, err
	}
	data, err := json.Marshal(normalized)
	if err != nil {
		return OrderPushSkipPolicy{}, fmt.Errorf("marshal order push skip config: %w", err)
	}
	if err := s.configDAO.SetValue(ctx, orderPushSkipConfigKey, string(data)); err != nil {
		return OrderPushSkipPolicy{}, err
	}
	return normalized, nil
}
