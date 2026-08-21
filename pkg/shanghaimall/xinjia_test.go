package shanghaimall

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPushXinjiaCenterSalesAndRefund(t *testing.T) {
	fixedNow := time.Date(2026, 8, 21, 14, 31, 0, 0, time.FixedZone("CST", 8*60*60))
	tests := []struct {
		name          string
		order         RetailOrder
		wantSaleDate  string
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
			wantSaleDate:  "2026-08-21T14:30:00.000+0800",
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
			wantSaleDate:  "2026-08-21T15:00:00.000+0800",
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
			wantSaleDate:  "2026-08-21T15:10:00.000+0800",
			wantSaleCount: -1,
			wantTotal:     -38,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var got xinjiaCenterSalesRequest
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/cre-agency-server/rest/sales/salesInput/create" {
					t.Errorf("request = %s %s", r.Method, r.URL.Path)
				}
				query := r.URL.Query()
				if query.Get("operator.id") != "xsjkcs" || query.Get("operator.fullname") != "销售接口传输" || query.Get("operator.namespace") != "mycompany.com" {
					t.Errorf("fixed query = %v", query)
				}
				if query.Get("time") != "2026-08-21T14:31:00.000+0800" {
					t.Errorf("query time = %q", query.Get("time"))
				}
				if r.Header.Get("Accept") != "application/json" || r.Header.Get("Content-Type") != "application/json" {
					t.Errorf("content headers = %q/%q", r.Header.Get("Accept"), r.Header.Get("Content-Type"))
				}
				if authorization := r.Header.Get("Authorization"); authorization != "Basic test-credential" {
					t.Errorf("authorization = %q", authorization)
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
				URL:           server.URL + "/cre-agency-server/rest/sales/salesInput/create?operator.id=xsjkcs&operator.fullname=销售接口传输&operator.namespace=mycompany.com",
				ProductCode:   "PRODUCT-001",
				StoreCode:     "STORE-001",
				TenantCode:    "00000018",
				Authorization: "Basic test-credential",
				Client:        server.Client(),
				Now:           func() time.Time { return fixedNow },
			}, test.order)
			if err != nil {
				t.Fatalf("pushXinjiaCenter returned error: %v", err)
			}
			if !result.Success || result.Target != TargetXinjiaCenter || result.HTTPStatus != http.StatusOK {
				t.Fatalf("result = %+v", result)
			}
			if got.ProductCode != "PRODUCT-001" || got.StoreCode != "STORE-001" || got.TenantCode != "00000018" {
				t.Fatalf("fixed codes = %q/%q/%q", got.ProductCode, got.StoreCode, got.TenantCode)
			}
			if got.BizState != "ineffect" || got.Effect || got.Receiver != "contract" {
				t.Fatalf("fixed body = bizState %q, effect %t, receiver %q", got.BizState, got.Effect, got.Receiver)
			}
			if got.SaleDate != test.wantSaleDate || got.Remark != test.order.DocNo {
				t.Fatalf("performance identity = saleDate %q, remark %q", got.SaleDate, got.Remark)
			}
			if got.SaleCount != test.wantSaleCount || got.SaleAmount != test.wantTotal {
				t.Fatalf("performance = saleCount %d, saleAmount %.2f", got.SaleCount, got.SaleAmount)
			}
			if len(got.Payments) != 1 || got.Payments[0].PaymentCode != "05" || got.Payments[0].Total != test.wantTotal {
				t.Fatalf("payments = %+v", got.Payments)
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
		URL:           server.URL + "/api/v1/sales/performance",
		ProductCode:   "PRODUCT-001",
		StoreCode:     "STORE-001",
		TenantCode:    "00000018",
		Authorization: "Basic test-credential",
		Client:        server.Client(),
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
		URL:           server.URL + "/api/v1/sales/performance",
		ProductCode:   "PRODUCT-001",
		StoreCode:     "STORE-001",
		TenantCode:    "00000018",
		Authorization: "Basic test-credential",
		Client:        server.Client(),
	}, RetailOrder{DocNo: "SALE-001", SaleTime: "2026-08-21 14:30:00", Amount: 10})
	if err == nil {
		t.Fatal("pushXinjiaCenter unexpectedly accepted business failure")
	}
	if result == nil || result.Success || result.HTTPStatus != http.StatusOK {
		t.Fatalf("result = %+v", result)
	}
}
