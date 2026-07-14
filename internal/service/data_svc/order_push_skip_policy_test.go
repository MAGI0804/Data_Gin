package data_svc

import "testing"

func TestOrderPushSkipPolicySkipsLastItemsInEachCycle(t *testing.T) {
	policy, err := (OrderPushSkipPolicy{Cycle: 5, Skip: 1}).Normalized()
	if err != nil {
		t.Fatalf("Normalize returned error: %v", err)
	}

	for position := 1; position <= 10; position++ {
		got := policy.ShouldSkip(position)
		want := position == 5 || position == 10
		if got != want {
			t.Fatalf("position %d skip = %v, want %v", position, got, want)
		}
	}
}

func TestOrderPushSkipPolicySupportsMultipleSkipsPerCycle(t *testing.T) {
	policy, err := (OrderPushSkipPolicy{Cycle: 5, Skip: 2}).Normalized()
	if err != nil {
		t.Fatalf("Normalize returned error: %v", err)
	}

	for position := 1; position <= 10; position++ {
		got := policy.ShouldSkip(position)
		want := position == 4 || position == 5 || position == 9 || position == 10
		if got != want {
			t.Fatalf("position %d skip = %v, want %v", position, got, want)
		}
	}
}

func TestOrderPushSkipPolicyDisabledWhenZero(t *testing.T) {
	policy, err := (OrderPushSkipPolicy{Cycle: 5, Skip: 0}).Normalized()
	if err != nil {
		t.Fatalf("Normalize returned error: %v", err)
	}
	if policy.ShouldSkip(5) {
		t.Fatal("disabled policy skipped an order")
	}
}

func TestOrderPushSkipPolicyRejectsSkipGreaterThanOrEqualCycle(t *testing.T) {
	if _, err := (OrderPushSkipPolicy{Cycle: 5, Skip: 5}).Normalized(); err == nil {
		t.Fatal("Normalize returned nil error, want validation error")
	}
}

func TestParseOrderPushSkipPolicyJSONReadsNestedDeliveryTaskConfig(t *testing.T) {
	policy, err := parseOrderPushSkipPolicyJSON(`{"push_skip_policy":{"cycle":5,"skip":1}}`)
	if err != nil {
		t.Fatalf("parseOrderPushSkipPolicyJSON returned error: %v", err)
	}
	if !policy.ShouldSkip(5) {
		t.Fatal("nested policy did not skip position 5")
	}
}
