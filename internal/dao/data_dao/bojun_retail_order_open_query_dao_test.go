package data_dao

import (
	"strings"
	"testing"

	"gorm.io/gorm"
)

func TestBojunRetailOrderDAOListOpenOrdersUsesBoundedSanitizedQuery(t *testing.T) {
	t.Parallel()
	db := dryRunWeatherDAOTestDB(t)
	var statement string
	if err := db.Callback().Query().After("gorm:query").Register("test:capture_open_bojun_orders_sql", func(tx *gorm.DB) {
		statement = tx.Statement.SQL.String()
	}); err != nil {
		t.Fatalf("register SQL capture callback: %v", err)
	}
	dao := NewBojunRetailOrderDAO(db)
	orders, err := dao.ListOpenOrders(t.Context(), OpenBojunOrderQuery{
		StartBillDate:  20260701,
		EndBillDate:    20260731,
		StoreCodes:     []string{"ABCN001P012"},
		OrderTypes:     []string{"CMR"},
		BeforeBillDate: 20260720,
		BeforeID:       99,
		Limit:          51,
	})
	if err != nil {
		t.Fatalf("ListOpenOrders() error=%v", err)
	}
	if orders == nil {
		t.Fatal("ListOpenOrders() returned nil slice")
	}
	for _, fragment := range []string{
		"SELECT `id`,`otherdocno`,`docno`,`billdate`,`c_store_code`,`c_store_name`",
		"billdate BETWEEN ? AND ?",
		"c_store_code IN (?)",
		"order_type_code IN (?)",
		"billdate < ? OR (billdate = ? AND id < ?)",
		"ORDER BY billdate DESC,id DESC",
		"LIMIT 51",
	} {
		if !strings.Contains(statement, fragment) {
			t.Fatalf("statement missing %q: %s", fragment, statement)
		}
	}
	for _, sensitive := range []string{"raw_content_json", "pay_items_json", "vipno", "raw_data_id"} {
		if strings.Contains(statement, sensitive) {
			t.Fatalf("statement selects sensitive column %q: %s", sensitive, statement)
		}
	}
	if strings.Contains(statement, "ABCN001P012") || strings.Contains(statement, "CMR") {
		t.Fatalf("statement interpolates filters: %s", statement)
	}
}

func TestBojunRetailOrderDAOListOpenOrdersRejectsUnboundedQuery(t *testing.T) {
	dao := &BojunRetailOrderDAO{}
	if _, err := dao.ListOpenOrders(t.Context(), OpenBojunOrderQuery{}); err == nil {
		t.Fatal("ListOpenOrders() accepted unbounded query")
	}
}
