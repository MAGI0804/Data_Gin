package shanghaimall

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPushXinjiaCenterSalesAndRefund(t *testing.T) {
	tests := []struct {
		name          string
		order         RetailOrder
		wantSaleCount int
		wantTotal     float64
	}{
		{
			name: "sale",
			order: RetailOrder{
				DocNo:         "SALE-001",
				OrderTypeCode: "CMR",
				SaleTime:      "2026-08-21 14:30:00",
				Amount:        128.50,
			},
			wantSaleCount: 1,
			wantTotal:     128.50,
		},
		{
			name: "refund with positive source amount",
			order: RetailOrder{
				DocNo:         "REFUND-001",
				OrderTypeCode: "RET",
				SaleTime:      "2026-08-21 15:00:00",
				Amount:        68,
			},
			wantSaleCount: -1,
			wantTotal:     -68,
		},
		{
			name: "refund with negative source amount",
			order: RetailOrder{
				DocNo:         "REFUND-002",
				OrderTypeCode: "RET",
				SaleTime:      "2026-08-21 15:10:00",
				Amount:        -38,
			},
			wantSaleCount: -1,
			wantTotal:     -38,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var got xinjiaCenterSalesRequest
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/api/v1/sales/performance" {
					t.Errorf("request = %s %s", r.Method, r.URL.Path)
				}
				if contentType := r.Header.Get("Content-Type"); contentType != "application/json" {
					t.Errorf("content type = %q", contentType)
				}
				if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
					t.Errorf("decode request: %v", err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"success":true}`))
			}))
			defer server.Close()

			result, err := pushXinjiaCenter(t.Context(), xinjiaCenterConfig{
				URL:         server.URL + "/api/v1/sales/performance",
				ProductCode: "PRODUCT-001",
				StoreCode:   "STORE-001",
				Client:      server.Client(),
			}, test.order)
			if err != nil {
				t.Fatalf("pushXinjiaCenter returned error: %v", err)
			}
			if !result.Success || result.Target != TargetXinjiaCenter || result.HTTPStatus != http.StatusOK {
				t.Fatalf("result = %+v", result)
			}
			if got.ProductCode != "PRODUCT-001" || got.StoreCode != "STORE-001" {
				t.Fatalf("fixed codes = %q/%q", got.ProductCode, got.StoreCode)
			}
			if got.TenantCode != test.order.SaleTime || got.Remark != test.order.DocNo {
				t.Fatalf("performance identity = tenantCode %q, remark %q", got.TenantCode, got.Remark)
			}
			if got.SaleCount != test.wantSaleCount || got.Total != test.wantTotal {
				t.Fatalf("performance = saleCount %d, total %.2f", got.SaleCount, got.Total)
			}
		})
	}
}

func TestPushXinjiaCenterRejectsIncompleteConfig(t *testing.T) {
	_, err := pushXinjiaCenter(t.Context(), xinjiaCenterConfig{}, RetailOrder{
		DocNo:    "SALE-001",
		SaleTime: "2026-08-21 14:30:00",
		Amount:   10,
	})
	if err == nil {
		t.Fatal("pushXinjiaCenter unexpectedly accepted incomplete config")
	}
}

func TestPushXinjiaCenterReturnsHTTPFailureResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"invalid sale"}`))
	}))
	defer server.Close()

	result, err := pushXinjiaCenter(t.Context(), xinjiaCenterConfig{
		URL:         server.URL + "/api/v1/sales/performance",
		ProductCode: "PRODUCT-001",
		StoreCode:   "STORE-001",
		Client:      server.Client(),
	}, RetailOrder{DocNo: "SALE-001", SaleTime: "2026-08-21 14:30:00", Amount: 10})
	if err == nil {
		t.Fatal("pushXinjiaCenter unexpectedly succeeded")
	}
	if result == nil || result.Success || result.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("result = %+v", result)
	}
}

func TestPushXinjiaCenterReturnsBusinessFailureResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":false,"message":"invalid sale"}`))
	}))
	defer server.Close()

	result, err := pushXinjiaCenter(t.Context(), xinjiaCenterConfig{
		URL:         server.URL + "/api/v1/sales/performance",
		ProductCode: "PRODUCT-001",
		StoreCode:   "STORE-001",
		Client:      server.Client(),
	}, RetailOrder{DocNo: "SALE-001", SaleTime: "2026-08-21 14:30:00", Amount: 10})
	if err == nil {
		t.Fatal("pushXinjiaCenter unexpectedly accepted business failure")
	}
	if result == nil || result.Success || result.HTTPStatus != http.StatusOK {
		t.Fatalf("result = %+v", result)
	}
}
