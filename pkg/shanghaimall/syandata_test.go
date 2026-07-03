package shanghaimall

import (
	"net/url"
	"testing"
)

func TestSyandataSignMatchesPythonAlgorithm(t *testing.T) {
	form := url.Values{}
	form.Set("method", "gogo.open.auto.routing")
	form.Set("timestamp", "20260703120000")
	form.Set("messageFormat", "json")
	form.Set("appKey", "test_app_key")
	form.Set("v", "1.0")
	form.Set("signMethod", "MD5")
	form.Set("lowerMethod", "com.gooagoo.exportbill")
	form.Set("appId", "test_app_id")
	form.Set("data", `{"billType":"1"}`)

	got := syandataSign("test_secret", form)
	if got != "7813B3AFB185373D3A2D8253D478DF99" {
		t.Fatalf("sign = %s", got)
	}
}

func TestRetailOrderFromBojunNormalAndRefund(t *testing.T) {
	normal := RetailOrder{
		DocNo:         "N001",
		OrderTypeCode: "CMR",
		SaleTime:      "2026-07-03 00:00:00",
		Amount:        100,
		ListAmount:    120,
	}
	if normal.IsRefund() {
		t.Fatal("normal order detected as refund")
	}
	if normal.normalizedQuantity() != 1 {
		t.Fatalf("normal quantity = %d", normal.normalizedQuantity())
	}

	refund := RetailOrder{
		DocNo:         "R001",
		OrderTypeCode: "RET",
		SaleTime:      "2026-07-03 00:00:00",
		Amount:        -50,
		ListAmount:    -60,
	}
	if !refund.IsRefund() {
		t.Fatal("refund order not detected")
	}
	if refund.normalizedQuantity() != -1 {
		t.Fatalf("refund quantity = %d", refund.normalizedQuantity())
	}
}
