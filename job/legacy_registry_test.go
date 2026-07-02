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
		"youzan_refund_fetch",
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
	if byCode["qimai_sales_push"].Category != "delivery" {
		t.Fatalf("qimai_sales_push category = %s, want delivery", byCode["qimai_sales_push"].Category)
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

func TestNewLegacyTaskRequiresRawDataIDForQimaiEnrich(t *testing.T) {
	_, err := NewLegacyTask("qimai_order_enrich", map[string]interface{}{})
	if err == nil {
		t.Fatal("NewLegacyTask returned nil error, want raw_data_id error")
	}
}
