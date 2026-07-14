package orderpush

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	TargetQimaiHangzhouHenglong = "qimai_hangzhou_henglong"
	TargetQimaiXian             = "qimai_xian"
	TargetBojunHangzhouHenglong = "bojun_hangzhou_henglong"
)

type SkipPolicy struct {
	Cycle int `json:"cycle"`
	Skip  int `json:"skip"`
}

type TargetSkipPolicy struct {
	TargetCode string `json:"target_code"`
	TargetName string `json:"target_name"`
	Cycle      int    `json:"cycle"`
	Skip       int    `json:"skip"`
}

type TargetOption struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type SkipConfig struct {
	Targets []TargetSkipPolicy `json:"targets"`
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

func (p TargetSkipPolicy) Normalized() (TargetSkipPolicy, error) {
	p.TargetCode = strings.TrimSpace(p.TargetCode)
	p.TargetName = strings.TrimSpace(p.TargetName)
	if p.TargetCode == "" {
		return TargetSkipPolicy{}, fmt.Errorf("target_code is required")
	}
	policy, err := (SkipPolicy{Cycle: p.Cycle, Skip: p.Skip}).Normalized()
	if err != nil {
		return TargetSkipPolicy{}, err
	}
	p.Cycle = policy.Cycle
	p.Skip = policy.Skip
	return p, nil
}

func (p TargetSkipPolicy) SkipPolicy() SkipPolicy {
	return SkipPolicy{Cycle: p.Cycle, Skip: p.Skip}
}

func (c SkipConfig) Normalized() (SkipConfig, error) {
	normalized := SkipConfig{Targets: make([]TargetSkipPolicy, 0, len(c.Targets))}
	seen := map[string]bool{}
	for _, target := range c.Targets {
		next, err := target.Normalized()
		if err != nil {
			return SkipConfig{}, err
		}
		key := strings.ToLower(next.TargetCode)
		if seen[key] {
			return SkipConfig{}, fmt.Errorf("duplicate target_code %q", next.TargetCode)
		}
		seen[key] = true
		normalized.Targets = append(normalized.Targets, next)
	}
	return normalized, nil
}

func (c SkipConfig) PolicyForTarget(targetCode string) SkipPolicy {
	targetCode = strings.TrimSpace(targetCode)
	if targetCode == "" {
		return SkipPolicy{}
	}
	for _, target := range c.Targets {
		if strings.EqualFold(strings.TrimSpace(target.TargetCode), targetCode) {
			return target.SkipPolicy()
		}
	}
	return SkipPolicy{}
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

func ParseSkipConfigJSON(raw string) (SkipConfig, error) {
	if raw == "" {
		return SkipConfig{}, nil
	}

	var cfg SkipConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return SkipConfig{}, err
	}
	if len(cfg.Targets) > 0 {
		return cfg.Normalized()
	}

	// Legacy runtime config used a top-level {cycle, skip}. It is intentionally
	// ignored here so a previous global setting cannot affect every target.
	var legacy SkipPolicy
	if err := json.Unmarshal([]byte(raw), &legacy); err != nil {
		return SkipConfig{}, err
	}
	if legacy.Cycle != 0 || legacy.Skip != 0 {
		return SkipConfig{}, nil
	}
	return cfg.Normalized()
}
