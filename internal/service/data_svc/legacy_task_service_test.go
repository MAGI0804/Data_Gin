package data_svc

import (
	"context"
	"testing"
)

func TestLegacyTaskServiceListDefinitionsHidesStoppedYouzanPushTasks(t *testing.T) {
	tasks := NewLegacyTaskService().ListDefinitions(context.Background())
	for _, task := range tasks {
		switch task.Code {
		case "youzan_sales_push", "youzan_refund_push":
			t.Fatalf("stopped youzan push task %s should not be listed", task.Code)
		}
	}
}

func TestLegacyTaskServiceRejectsStoppedYouzanPushTasks(t *testing.T) {
	_, err := NewLegacyTaskService().Enqueue(context.Background(), "youzan_sales_push", nil)
	if err == nil {
		t.Fatal("Enqueue returned nil error for stopped youzan_sales_push")
	}
}
