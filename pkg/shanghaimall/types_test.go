package shanghaimall

import (
	"testing"

	"gin-biz-web-api/model"
)

func TestRetailOrderFromBojunUsesPushAmountOnlyForOracleOrders(t *testing.T) {
	retailID := uint64(45077)
	oracleOrder := RetailOrderFromBojun(model.BojunRetailOrder{
		OracleRetailID: &retailID, DocNo: "ORACLE-1", BillDate: 20260826,
		OrderTypeCode: "CMR", TotalAmtActual: 470.83, TotalAmtList: 470.83, PushAmount: 0,
	})
	if oracleOrder.Amount != 0 || oracleOrder.ListAmount != 0 {
		t.Fatalf("Oracle amount/list amount = %.2f/%.2f, want 0/0", oracleOrder.Amount, oracleOrder.ListAmount)
	}

	apiOrder := RetailOrderFromBojun(model.BojunRetailOrder{
		DocNo: "API-1", BillDate: 20260826, OrderTypeCode: "CMR",
		TotalAmtActual: 88.8, TotalAmtList: 100, PushAmount: 0,
	})
	if apiOrder.Amount != 88.8 || apiOrder.ListAmount != 100 {
		t.Fatalf("API amount/list amount = %.2f/%.2f", apiOrder.Amount, apiOrder.ListAmount)
	}
}

func TestRetailOrderFromBojunSignsOracleRefundPushAmount(t *testing.T) {
	retailID := uint64(45078)
	order := RetailOrderFromBojun(model.BojunRetailOrder{
		OracleRetailID: &retailID, DocNo: "ORACLE-RET", BillDate: 20260826,
		OrderTypeCode: "RET", PushAmount: 39.9,
	})
	if order.Amount != -39.9 || order.ListAmount != -39.9 {
		t.Fatalf("refund amount/list amount = %.2f/%.2f", order.Amount, order.ListAmount)
	}
}
