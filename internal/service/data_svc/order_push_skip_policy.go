package data_svc

import (
	"encoding/json"
	"fmt"
)

const orderPushSkipConfigKey = "order_push_skip_policy"

type OrderPushSkipPolicy struct {
	Cycle int `json:"cycle"`
	Skip  int `json:"skip"`
}

func (p OrderPushSkipPolicy) Normalized() (OrderPushSkipPolicy, error) {
	if p.Cycle < 0 || p.Skip < 0 {
		return OrderPushSkipPolicy{}, fmt.Errorf("cycle and skip must be greater than or equal to 0")
	}
	if p.Cycle == 0 || p.Skip == 0 {
		return OrderPushSkipPolicy{}, nil
	}
	if p.Skip >= p.Cycle {
		return OrderPushSkipPolicy{}, fmt.Errorf("skip must be less than cycle")
	}
	return p, nil
}

func (p OrderPushSkipPolicy) Enabled() bool {
	return p.Cycle > 0 && p.Skip > 0 && p.Skip < p.Cycle
}

func (p OrderPushSkipPolicy) ShouldSkip(position int) bool {
	if !p.Enabled() || position <= 0 {
		return false
	}
	slot := position % p.Cycle
	if slot == 0 {
		slot = p.Cycle
	}
	return slot > p.Cycle-p.Skip
}

func (p OrderPushSkipPolicy) Reason(position int) string {
	return fmt.Sprintf("按少推送规则跳过: 每 %d 单少推 %d 单，当前第 %d 单", p.Cycle, p.Skip, position)
}

func parseOrderPushSkipPolicyJSON(raw string) (OrderPushSkipPolicy, error) {
	if raw == "" {
		return OrderPushSkipPolicy{}, nil
	}

	var direct OrderPushSkipPolicy
	if err := json.Unmarshal([]byte(raw), &direct); err != nil {
		return OrderPushSkipPolicy{}, err
	}
	if direct.Cycle != 0 || direct.Skip != 0 {
		return direct.Normalized()
	}

	var nested struct {
		PushSkipPolicy OrderPushSkipPolicy `json:"push_skip_policy"`
		OrderPushSkip  OrderPushSkipPolicy `json:"order_push_skip"`
	}
	if err := json.Unmarshal([]byte(raw), &nested); err != nil {
		return OrderPushSkipPolicy{}, err
	}
	if nested.PushSkipPolicy.Cycle != 0 || nested.PushSkipPolicy.Skip != 0 {
		return nested.PushSkipPolicy.Normalized()
	}
	return nested.OrderPushSkip.Normalized()
}
