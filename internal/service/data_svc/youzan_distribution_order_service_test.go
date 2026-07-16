package data_svc

import (
	"context"
	"fmt"
	"testing"

	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/logger"

	"go.uber.org/zap"
)

type fakeYouzanDistributionClient struct {
	pages        [][]map[string]any
	decryptSizes []int
}

func (f *fakeYouzanDistributionClient) FetchOrderPage(_ context.Context, _ string, _, _ string, pageNo, _ int) ([]map[string]any, bool, error) {
	if pageNo > len(f.pages) {
		return nil, false, nil
	}
	return f.pages[pageNo-1], pageNo < len(f.pages), nil
}

func (f *fakeYouzanDistributionClient) DecryptBatch(_ context.Context, _ string, sources []string) ([]string, error) {
	f.decryptSizes = append(f.decryptSizes, len(sources))
	result := make([]string, len(sources))
	for i, source := range sources {
		result[i] = "decrypted:" + source
	}
	return result, nil
}

type fakeYouzanDistributionWriter struct {
	batches [][]model.YouzanDistributionOrder
}

func (f *fakeYouzanDistributionWriter) CreateBatchIfNotExists(_ context.Context, orders []model.YouzanDistributionOrder) (int64, error) {
	batch := append([]model.YouzanDistributionOrder(nil), orders...)
	f.batches = append(f.batches, batch)
	return int64(len(orders)), nil
}

func TestYouzanDistributionOrderServiceSyncsEveryPageAndDecryptsNickname(t *testing.T) {
	initYouzanDistributionTestLogger()
	client := &fakeYouzanDistributionClient{pages: [][]map[string]any{
		{distributionOrderFixture("D100", "encrypted-1")},
		{distributionOrderFixture("D200", "encrypted-2")},
	}}
	writer := &fakeYouzanDistributionWriter{}
	service := newYouzanDistributionOrderService(client, writer, func() (string, error) { return "token", nil })

	result, err := service.SyncRange(context.Background(), "2026-07-05 00:00:00", "2026-07-05 23:59:59")
	if err != nil {
		t.Fatalf("SyncRange() error = %v", err)
	}
	if result.FetchPages != 2 || result.TotalCount != 2 || result.SavedCount != 2 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(writer.batches) != 2 || writer.batches[1][0].TID != "D200" {
		t.Fatalf("expected one persisted batch per page, got %#v", writer.batches)
	}
	if got := writer.batches[0][0].FansNickname; got != "decrypted:encrypted-1" {
		t.Fatalf("FansNickname = %q", got)
	}
	if got := writer.batches[0][0].FansNicknameEncrypted; got != "encrypted-1" {
		t.Fatalf("FansNicknameEncrypted = %q", got)
	}
}

func TestYouzanDistributionOrderServiceSplitsDecryptRequestsAtTenThousand(t *testing.T) {
	initYouzanDistributionTestLogger()
	orders := make([]map[string]any, 10001)
	for i := range orders {
		orders[i] = distributionOrderFixture(fmt.Sprintf("D%d", i), fmt.Sprintf("$encrypted-%d$", i))
	}
	client := &fakeYouzanDistributionClient{pages: [][]map[string]any{orders}}
	service := newYouzanDistributionOrderService(client, &fakeYouzanDistributionWriter{}, func() (string, error) { return "token", nil })

	if _, err := service.SyncRange(context.Background(), "2026-07-05 00:00:00", "2026-07-05 23:59:59"); err != nil {
		t.Fatalf("SyncRange() error = %v", err)
	}
	if fmt.Sprint(client.decryptSizes) != "[10000 1]" {
		t.Fatalf("decrypt batch sizes = %v, want [10000 1]", client.decryptSizes)
	}
}

func initYouzanDistributionTestLogger() {
	if logger.Logger == nil {
		logger.Logger = zap.NewNop()
	}
}

func distributionOrderFixture(tid, nickname string) map[string]any {
	return map[string]any{
		"order_info": map[string]any{
			"tid":          tid,
			"status":       "TRADE_SUCCESS",
			"success_time": "2026-07-05 12:00:00",
		},
		"buyer_info": map[string]any{"fans_nickname": nickname},
		"orders":     []any{map[string]any{"title": "test"}},
	}
}
