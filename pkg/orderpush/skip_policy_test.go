package orderpush

import "testing"

func TestSkipConfigPolicyForTarget(t *testing.T) {
	cfg, err := (SkipConfig{Targets: []TargetSkipPolicy{
		{TargetCode: "hangzhou_henglong", Cycle: 5, Skip: 1},
		{TargetCode: "qiantan", Cycle: 10, Skip: 2},
	}}).Normalized()
	if err != nil {
		t.Fatalf("Normalize returned error: %v", err)
	}

	if !cfg.PolicyForTarget("hangzhou_henglong").ShouldSkip(5) {
		t.Fatal("hangzhou_henglong policy did not skip position 5")
	}
	if cfg.PolicyForTarget("qiantan").ShouldSkip(5) {
		t.Fatal("qiantan policy unexpectedly skipped position 5")
	}
	if cfg.PolicyForTarget("unknown").ShouldSkip(5) {
		t.Fatal("unknown target unexpectedly inherited a policy")
	}
}

func TestParseSkipConfigJSONIgnoresLegacyGlobalPolicy(t *testing.T) {
	cfg, err := ParseSkipConfigJSON(`{"cycle":5,"skip":1}`)
	if err != nil {
		t.Fatalf("ParseSkipConfigJSON returned error: %v", err)
	}
	if cfg.PolicyForTarget("hangzhou_henglong").ShouldSkip(5) {
		t.Fatal("legacy global policy affected a target")
	}
}

func TestSkipConfigRejectsDuplicateTargets(t *testing.T) {
	_, err := (SkipConfig{Targets: []TargetSkipPolicy{
		{TargetCode: "qiantan", Cycle: 5, Skip: 1},
		{TargetCode: "QIANTAN", Cycle: 5, Skip: 1},
	}}).Normalized()
	if err == nil {
		t.Fatal("Normalize returned nil error, want duplicate target error")
	}
}

func TestSkipPolicyRejectsSkipWithoutCycle(t *testing.T) {
	if _, err := (SkipPolicy{Cycle: 0, Skip: 1}).Normalized(); err == nil {
		t.Fatal("Normalize returned nil error, want validation error")
	}
}
