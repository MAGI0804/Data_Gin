package data_dao

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestDeliveryLogDAORejectsInvalidWeatherBatchOperationsBeforeDatabase(t *testing.T) {
	t.Parallel()
	dao := &DeliveryLogDAO{}
	if _, err := dao.FindLatestWeatherBatch(context.Background(), 1, 2, "hourly", 1); err == nil {
		t.Fatal("FindLatestWeatherBatch() accepted unavailable database")
	}
	if err := dao.FinishWeatherBatch(context.Background(), 1, DeliveryLogBatchFinish{
		Status: "success", Success: true, FinishedAt: time.Now(),
	}); err == nil {
		t.Fatal("FinishWeatherBatch() accepted unavailable database")
	}
	if err := dao.ReconcileWeatherBatchSuccess(
		context.Background(), 1, strings.Repeat("a", 64), 2, 3, time.Now(),
	); err == nil {
		t.Fatal("ReconcileWeatherBatchSuccess() accepted unavailable database")
	}
}

func TestDeliveryLogDAORejectsInconsistentWeatherBatchCompletion(t *testing.T) {
	t.Parallel()
	tests := []DeliveryLogBatchFinish{
		{Status: "running", FinishedAt: time.Now()},
		{Status: "success", Success: false, FinishedAt: time.Now()},
		{Status: "failed", Success: true, FinishedAt: time.Now()},
		{Status: "unknown", Success: true, FinishedAt: time.Now()},
		{Status: "failed", HTTPStatus: -1, FinishedAt: time.Now()},
		{Status: "failed", FeishuCode: -1, FinishedAt: time.Now()},
		{Status: "failed", RowStart: 2, RowEnd: 0, FinishedAt: time.Now()},
		{Status: "failed"},
	}
	for _, finish := range tests {
		finish.Status = strings.TrimSpace(finish.Status)
		if validDeliveryLogBatchFinish(finish) {
			t.Fatalf("validDeliveryLogBatchFinish() accepted %+v", finish)
		}
	}
}

func TestDeliveryLogBatchCompletionAcceptsTerminalStates(t *testing.T) {
	t.Parallel()
	now := time.Now()
	for _, finish := range []DeliveryLogBatchFinish{
		{Status: "success", Success: true, HTTPStatus: 200, FinishedAt: now},
		{Status: "failed", HTTPStatus: 403, FeishuCode: 91, FinishedAt: now},
		{Status: "unknown", HTTPStatus: 503, SafeError: "provider unavailable", FinishedAt: now},
	} {
		if !validDeliveryLogBatchFinish(finish) {
			t.Fatalf("validDeliveryLogBatchFinish() rejected %+v", finish)
		}
	}
}
