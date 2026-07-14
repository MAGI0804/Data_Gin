package Trigger

import (
	"context"
	"strings"
	"testing"

	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/orderpush"
)

type fakeQimaiDeliveryLogCreator struct {
	logs []model.DeliveryLog
}

func (f *fakeQimaiDeliveryLogCreator) Create(ctx context.Context, log *model.DeliveryLog) (uint, error) {
	_ = ctx
	f.logs = append(f.logs, *log)
	return uint(len(f.logs)), nil
}

func TestWriteQimaiSkippedLogRecordsPolicySkipAsSuccessfulDeliveryLog(t *testing.T) {
	logCreator := &fakeQimaiDeliveryLogCreator{}
	trigger := &SalesSyncTrigger{logDAO: logCreator}
	policy := orderpush.SkipPolicy{Cycle: 5, Skip: 1}

	trigger.writeQimaiSkippedLog(context.Background(), "trace-1", 9, 11, "Q005", policy, 5)

	if len(logCreator.logs) != 1 {
		t.Fatalf("logs length = %d, want 1", len(logCreator.logs))
	}
	log := logCreator.logs[0]
	if !log.Success {
		t.Fatal("skip log success = false, want true")
	}
	if log.SourceCode != "qimai_order" || log.DestinationCode != "hangzhou_henglong" {
		t.Fatalf("log source/destination = %s/%s", log.SourceCode, log.DestinationCode)
	}
	if log.ResponseBody != "skipped_by_order_push_policy" {
		t.Fatalf("response body = %s", log.ResponseBody)
	}
	if !strings.Contains(log.ErrorMessage, "每 5 单少推 1 单") {
		t.Fatalf("error message = %s", log.ErrorMessage)
	}
}
