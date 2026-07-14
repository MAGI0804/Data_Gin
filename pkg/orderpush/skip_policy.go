package orderpush

import (
	"encoding/json"
	"fmt"
)

type SkipPolicy struct {
	Cycle int `json:"cycle"`
	Skip  int `json:"skip"`
}

func (p SkipPolicy) Normalized() (SkipPolicy, error) {
	if p.Cycle < 0 || p.Skip < 0 {
		return SkipPolicy{}, fmt.Errorf("cycle and skip must be greater than or equal to 0")
	}
	if p.Cycle == 0 || p.Skip == 0 {
		return SkipPolicy{}, nil
	}
	if p.Skip >= p.Cycle {
		return SkipPolicy{}, fmt.Errorf("skip must be less than cycle")
	}
	return p, nil
}

func (p SkipPolicy) Enabled() bool {
	return p.Cycle > 0 && p.Skip > 0 && p.Skip < p.Cycle
}

func (p SkipPolicy) ShouldSkip(position int) bool {
	if !p.Enabled() || position <= 0 {
		return false
	}
	slot := position % p.Cycle
	if slot == 0 {
		slot = p.Cycle
	}
	return slot > p.Cycle-p.Skip
}

func (p SkipPolicy) Reason(position int) string {
	return fmt.Sprintf("按少推送规则跳过: 每 %d 单少推 %d 单，当前第 %d 单", p.Cycle, p.Skip, position)
}

func ParseSkipPolicyJSON(raw string) (SkipPolicy, error) {
	if raw == "" {
		return SkipPolicy{}, nil
	}

	var direct SkipPolicy
	if err := json.Unmarshal([]byte(raw), &direct); err != nil {
		return SkipPolicy{}, err
	}
	if direct.Cycle != 0 || direct.Skip != 0 {
		return direct.Normalized()
	}

	var nested struct {
		PushSkipPolicy SkipPolicy `json:"push_skip_policy"`
		OrderPushSkip  SkipPolicy `json:"order_push_skip"`
	}
	if err := json.Unmarshal([]byte(raw), &nested); err != nil {
		return SkipPolicy{}, err
	}
	if nested.PushSkipPolicy.Cycle != 0 || nested.PushSkipPolicy.Skip != 0 {
		return nested.PushSkipPolicy.Normalized()
	}
	return nested.OrderPushSkip.Normalized()
}
