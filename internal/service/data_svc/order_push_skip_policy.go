package data_svc

import (
	"context"

	"gin-biz-web-api/pkg/orderpush"
)

const orderPushSkipConfigKey = "order_push_skip_policy"

type OrderPushSkipPolicy = orderpush.SkipPolicy

type orderPushSkipPolicyGetter interface {
	Get(ctx context.Context) (OrderPushSkipPolicy, error)
}

func parseOrderPushSkipPolicyJSON(raw string) (OrderPushSkipPolicy, error) {
	return orderpush.ParseSkipPolicyJSON(raw)
}
