package data_dao

import (
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestBojunRetailOrderDAOUpdateCompletedAtIfEmptyUsesNarrowUpdate(t *testing.T) {
	t.Parallel()
	db := dryRunWeatherDAOTestDB(t)
	var statement string
	if err := db.Callback().Update().After("gorm:update").Register("test:capture_bojun_completed_at_sql", func(tx *gorm.DB) {
		statement = tx.Statement.SQL.String()
	}); err != nil {
		t.Fatalf("register SQL capture callback: %v", err)
	}
	updated, err := NewBojunRetailOrderDAO(db).UpdateCompletedAtIfEmpty(
		t.Context(),
		"B001",
		time.Date(2026, 7, 11, 10, 31, 22, 0, time.FixedZone("CST", 8*60*60)),
	)
	if err != nil {
		t.Fatalf("UpdateCompletedAtIfEmpty() error=%v", err)
	}
	if updated {
		t.Fatal("dry-run update unexpectedly reported rows affected")
	}
	for _, fragment := range []string{
		"UPDATE `bojun_retail_orders`",
		"SET `completed_at`=?,`updated_at`=?",
		"docno = ? AND completed_at IS NULL",
	} {
		if !strings.Contains(statement, fragment) {
			t.Fatalf("statement missing %q: %s", fragment, statement)
		}
	}
	for _, forbidden := range []string{"synced", "matched_docno", "raw_data_id"} {
		if strings.Contains(statement, forbidden) {
			t.Fatalf("statement updates %q: %s", forbidden, statement)
		}
	}
}

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

func TestBojunRetailOrderDAOCountOpenOrdersIgnoresCursorBoundary(t *testing.T) {
	t.Parallel()
	db := dryRunWeatherDAOTestDB(t)
	var statement string
	if err := db.Callback().Query().After("gorm:query").Register("test:capture_open_bojun_count_sql", func(tx *gorm.DB) {
		statement = tx.Statement.SQL.String()
	}); err != nil {
		t.Fatalf("register SQL capture callback: %v", err)
	}
	_, err := NewBojunRetailOrderDAO(db).CountOpenOrders(t.Context(), OpenBojunOrderQuery{
		StartBillDate: 20260701, EndBillDate: 20260731, StoreCodes: []string{"ABCN001P012"},
		OrderTypes: []string{"CMR"}, BeforeBillDate: 20260720, BeforeID: 99,
	})
	if err != nil {
		t.Fatalf("CountOpenOrders() error=%v", err)
	}
	for _, fragment := range []string{"SELECT count(*)", "billdate BETWEEN ? AND ?", "c_store_code IN (?)", "order_type_code IN (?)"} {
		if !strings.Contains(statement, fragment) {
			t.Fatalf("statement missing %q: %s", fragment, statement)
		}
	}
	if strings.Contains(statement, "billdate < ?") || strings.Contains(statement, "LIMIT") || strings.Contains(statement, "ORDER BY") {
		t.Fatalf("count statement contains page boundary: %s", statement)
	}
}
