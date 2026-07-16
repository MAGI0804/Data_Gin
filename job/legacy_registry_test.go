package job

import "testing"

func TestLegacyTaskDefinitionsExposeConfiguredFetchAndDeliveryJobs(t *testing.T) {
	definitions := LegacyTaskDefinitions()
	byCode := make(map[string]LegacyTaskDefinition, len(definitions))
	for _, definition := range definitions {
		byCode[definition.Code] = definition
	}

	for _, code := range []string{
		"youzan_order_fetch",
		"youzan_distribution_order_fetch",
		"youzan_refund_fetch",
		"bojun_order_fetch",
		"youzan_sales_push",
		"youzan_refund_push",
		"qimai_sales_push",
		"xian_order_push",
		"qimai_order_enrich",
	} {
		if _, ok := byCode[code]; !ok {
			t.Fatalf("legacy definition %s not found", code)
		}
	}

	if byCode["youzan_order_fetch"].Category != "fetch" {
		t.Fatalf("youzan_order_fetch category = %s, want fetch", byCode["youzan_order_fetch"].Category)
	}
	if byCode["youzan_distribution_order_fetch"].OutputTable != "youzan_distribution_orders" {
		t.Fatalf("youzan distribution output table = %s", byCode["youzan_distribution_order_fetch"].OutputTable)
	}
	if byCode["youzan_distribution_order_fetch"].CronExpr == "" {
		t.Fatal("youzan distribution fetch must expose its daily cron expression")
	}
	if byCode["bojun_order_fetch"].TaskType != TypeBojunOrderFetch {
		t.Fatalf("bojun_order_fetch task type = %s, want %s", byCode["bojun_order_fetch"].TaskType, TypeBojunOrderFetch)
	}
	if byCode["qimai_sales_push"].Category != "delivery" {
		t.Fatalf("qimai_sales_push category = %s, want delivery", byCode["qimai_sales_push"].Category)
	}
	if byCode["qimai_order_enrich"].SourceCode != "qimai_order" {
		t.Fatalf("qimai_order_enrich source_code = %s, want qimai_order", byCode["qimai_order_enrich"].SourceCode)
	}
	if byCode["qimai_order_enrich"].OutputTable != "qimai_order_data" {
		t.Fatalf("qimai_order_enrich output_table = %s, want qimai_order_data", byCode["qimai_order_enrich"].OutputTable)
	}
}

func TestLegacyTransformRuleDefinitionsExposeExistingHardcodedRules(t *testing.T) {
	definitions := LegacyTransformRuleDefinitions()
	byCode := make(map[string]LegacyTransformRuleDefinition, len(definitions))
	for _, definition := range definitions {
		byCode[definition.Code] = definition
	}

	for _, code := range []string{
		"qimai_order_http_enrich",
		"youzan_order_direct_store",
		"youzan_refund_direct_store",
	} {
		if _, ok := byCode[code]; !ok {
			t.Fatalf("legacy transform definition %s not found", code)
		}
	}

	if byCode["qimai_order_http_enrich"].Handler != "Trigger/qimai_order_trigger.go" {
		t.Fatalf("qimai_order_http_enrich handler = %s", byCode["qimai_order_http_enrich"].Handler)
	}
}

func TestNewLegacyTaskBuildsRegisteredTasks(t *testing.T) {
	task, err := NewLegacyTask("qimai_sales_push", map[string]interface{}{
		"shop_code":      "SHOP-1",
		"status":         "70",
		"store_code":     "STORE-1",
		"mall_item_code": "ITEM-1",
	})
	if err != nil {
		t.Fatalf("NewLegacyTask returned error: %v", err)
	}
	if task.Type() != TypeSalesSync {
		t.Fatalf("task type = %s, want %s", task.Type(), TypeSalesSync)
	}
}

func TestNewLegacyTaskBuildsBojunOrderFetchTask(t *testing.T) {
	task, err := NewLegacyTask("bojun_order_fetch", map[string]interface{}{
		"start_time": "2026-07-08 10:00:00",
		"end_time":   "2026-07-08 11:00:00",
	})
	if err != nil {
		t.Fatalf("NewLegacyTask returned error: %v", err)
	}
	if task.Type() != TypeBojunOrderFetch {
		t.Fatalf("task type = %s, want %s", task.Type(), TypeBojunOrderFetch)
	}
}

func TestNewLegacyTaskBuildsYouzanDistributionFetchTask(t *testing.T) {
	task, err := NewLegacyTask("youzan_distribution_order_fetch", map[string]interface{}{
		"start_time": "2026-07-05 00:00:00",
		"end_time":   "2026-07-05 23:59:59",
	})
	if err != nil {
		t.Fatalf("NewLegacyTask returned error: %v", err)
	}
	if task.Type() != TypeYouzanDistributionOrderSync {
		t.Fatalf("task type = %s, want %s", task.Type(), TypeYouzanDistributionOrderSync)
	}
}

func TestNewLegacyTaskRequiresBojunOrderTimeRange(t *testing.T) {
	_, err := NewLegacyTask("bojun_order_fetch", map[string]interface{}{})
	if err == nil {
		t.Fatal("NewLegacyTask returned nil error, want time range error")
	}
}

func TestNewLegacyTaskRequiresRawDataIDForQimaiEnrich(t *testing.T) {
	_, err := NewLegacyTask("qimai_order_enrich", map[string]interface{}{})
	if err == nil {
		t.Fatal("NewLegacyTask returned nil error, want raw_data_id error")
	}
}

func TestScheduledLegacyTaskDefinitionsExcludeStoppedYouzanTriggers(t *testing.T) {
	definitions := ScheduledLegacyTaskDefinitions()
	foundDistributionFetch := false
	for _, definition := range definitions {
		if definition.Code == "youzan_distribution_order_fetch" {
			foundDistributionFetch = true
		}
		switch definition.Code {
		case "youzan_order_fetch", "youzan_refund_fetch", "youzan_sales_push", "youzan_refund_push":
			t.Fatalf("stopped youzan trigger %s should not be scheduled", definition.Code)
		}
	}
	if !foundDistributionFetch {
		t.Fatal("scheduled definitions must include youzan distribution order fetch")
	}
}
