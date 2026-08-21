package config_svc

import (
	"context"
	"testing"

	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/orderpush"
)

type fakePushDestinationLister struct {
	destinations []model.DestinationDefinition
}

func (f fakePushDestinationLister) FindAll(ctx context.Context) ([]model.DestinationDefinition, error) {
	_ = ctx
	return f.destinations, nil
}

func TestListTargetsMergesBuiltinAndConfiguredDestinations(t *testing.T) {
	service := &OrderPushSkipConfigService{
		destinationDAO: fakePushDestinationLister{destinations: []model.DestinationDefinition{
			{Name: "自定义目标", Code: "custom_target"},
			{Name: "重复前滩", Code: "qiantan"},
		}},
	}

	targets, err := service.ListTargets(context.Background())
	if err != nil {
		t.Fatalf("ListTargets returned error: %v", err)
	}

	if !hasTarget(targets, orderpush.TargetQimaiHangzhouHenglong) {
		t.Fatal("targets missing qimai hangzhou")
	}
	if !hasTarget(targets, orderpush.TargetQimaiXian) {
		t.Fatal("targets missing qimai xian")
	}
	if !hasTarget(targets, orderpush.TargetBojunHangzhouHenglong) {
		t.Fatal("targets missing bojun hangzhou")
	}
	if !hasTarget(targets, "xinjia_center") {
		t.Fatal("targets missing xinjia center")
	}
	if !hasTarget(targets, "custom_target") {
		t.Fatal("targets missing custom destination")
	}

	countQiantan := 0
	for _, target := range targets {
		if target.Code == "qiantan" {
			countQiantan++
		}
	}
	if countQiantan != 1 {
		t.Fatalf("qiantan count = %d, want 1", countQiantan)
	}
}

func hasTarget(targets []orderpush.TargetOption, code string) bool {
	for _, target := range targets {
		if target.Code == code {
			return true
		}
	}
	return false
}
