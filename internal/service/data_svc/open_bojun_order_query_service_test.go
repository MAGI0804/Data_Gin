package data_svc

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/model"
)

type fakeOpenBojunOrderReader struct {
	query  data_dao.OpenBojunOrderQuery
	orders []model.BojunRetailOrder
	calls  int
}

func (reader *fakeOpenBojunOrderReader) ListOpenOrders(
	_ context.Context,
	query data_dao.OpenBojunOrderQuery,
) ([]model.BojunRetailOrder, error) {
	reader.calls++
	reader.query = query
	return reader.orders, nil
}

type fakeOpenBojunPermissionReader struct {
	allowed    bool
	permission string
}

func (reader *fakeOpenBojunPermissionReader) HasPermission(
	_ context.Context,
	_ uint,
	permission string,
	_ time.Time,
) (bool, error) {
	reader.permission = permission
	return reader.allowed, nil
}

func TestOpenBojunOrderQueryServiceReturnsSanitizedCursorPage(t *testing.T) {
	orders := &fakeOpenBojunOrderReader{orders: []model.BojunRetailOrder{
		{
			BaseModel: model.BaseModel{ID: 9}, DocNo: "B001", OtherDocNo: "EXT001",
			BillDate: 20260703, StoreCode: "ABCN001P012", StoreName: "前滩",
			OrderTypeCode: "CMR", OrderTypeName: "正常零售", TotalLines: 1, TotalQty: 2,
			TotalAmtList: 500, TotalAmtActual: 446.4, AvgDiscount: 0.8928,
			ItemsJSON:      `[{"no":"SKU001","mProductName":"商品","qty":2,"totAmtActual":446.4,"vipno":"secret"}]`,
			RawContentJSON: `{"secret":"must-not-leak"}`, VIPNo: "member-secret",
		},
		{BaseModel: model.BaseModel{ID: 8}, DocNo: "B002", BillDate: 20260702},
	}}
	permissions := &fakeOpenBojunPermissionReader{allowed: true}
	service := newOpenBojunOrderQueryService(orders, permissions, time.Now)
	result, err := service.Query(t.Context(), 17, requestbody.OpenBojunOrderQueryRequest{
		StartDate: "2026-07-01", EndDate: "2026-07-31",
		StoreCodes: []string{" abcn001p012 ", "ABCN001P012"},
		OrderTypes: []string{"cmr"}, PageSize: 1,
	})
	if err != nil {
		t.Fatalf("Query() error=%v", err)
	}
	if permissions.permission != model.PermissionBojunOrderRead || orders.calls != 1 {
		t.Fatalf("permission=%q calls=%d", permissions.permission, orders.calls)
	}
	if orders.query.StartBillDate != 20260701 || orders.query.EndBillDate != 20260731 ||
		len(orders.query.StoreCodes) != 1 || orders.query.StoreCodes[0] != "ABCN001P012" ||
		orders.query.Limit != 2 {
		t.Fatalf("query=%+v", orders.query)
	}
	if len(result.Items) != 1 || result.Items[0].OrderDate != "2026-07-03 00:00:00" ||
		result.Items[0].ActualAmount != "446.40" || len(result.Items[0].Items) != 1 ||
		result.Items[0].Items[0].SKUNo != "SKU001" {
		t.Fatalf("result=%+v", result)
	}
	if !result.Pagination.HasMore || result.Pagination.NextCursor == "" {
		t.Fatalf("pagination=%+v", result.Pagination)
	}
	payload, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	for _, sensitive := range []string{"member-secret", "must-not-leak", "vipno"} {
		if strings.Contains(string(payload), sensitive) {
			t.Fatalf("response leaked %q: %s", sensitive, payload)
		}
	}
	cursor, err := decodeOpenBojunOrderCursor(result.Pagination.NextCursor)
	if err != nil || cursor.BillDate != 20260703 || cursor.ID != 9 {
		t.Fatalf("cursor=%+v error=%v", cursor, err)
	}
}

func TestOpenBojunOrderQueryServiceRejectsInvalidFiltersBeforeDAO(t *testing.T) {
	orders := &fakeOpenBojunOrderReader{}
	service := newOpenBojunOrderQueryService(
		orders,
		&fakeOpenBojunPermissionReader{allowed: true},
		time.Now,
	)
	tests := []requestbody.OpenBojunOrderQueryRequest{
		{StartDate: "2026-07-01", EndDate: "2026-07-31"},
		{StartDate: "2026-07-31", EndDate: "2026-07-01", StoreCodes: []string{"ABCN001P012"}},
		{StartDate: "2026-07-01", EndDate: "2026-08-01", StoreCodes: []string{"ABCN001P012"}},
		{StartDate: "2026-07-01", EndDate: "2026-07-31", StoreCodes: []string{"bad code"}},
		{StartDate: "2026-07-01", EndDate: "2026-07-31", StoreCodes: []string{"ABCN001P012"}, OrderTypes: []string{"OTHER"}},
		{StartDate: "2026-07-01", EndDate: "2026-07-31", StoreCodes: []string{"ABCN001P012"}, PageSize: 101},
	}
	for _, request := range tests {
		if _, err := service.Query(t.Context(), 17, request); err == nil {
			t.Fatalf("Query(%+v) error=nil", request)
		}
	}
	if orders.calls != 0 {
		t.Fatalf("DAO calls=%d", orders.calls)
	}
}

func TestOpenBojunOrderQueryServiceRequiresDedicatedPermission(t *testing.T) {
	orders := &fakeOpenBojunOrderReader{}
	service := newOpenBojunOrderQueryService(
		orders,
		&fakeOpenBojunPermissionReader{allowed: false},
		time.Now,
	)
	_, err := service.Query(t.Context(), 17, requestbody.OpenBojunOrderQueryRequest{})
	if err == nil || !errors.Is(err, ErrOpenBojunOrderForbidden) || orders.calls != 0 {
		t.Fatalf("error=%v DAO calls=%d", err, orders.calls)
	}
}

func TestOpenBojunOrderLinesFailsClosedOnInvalidJSON(t *testing.T) {
	items := openBojunOrderLines(`{"vipno":"secret"}`)
	if items == nil || len(items) != 0 {
		t.Fatalf("items=%+v", items)
	}
}
