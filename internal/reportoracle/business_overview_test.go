package reportoracle

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
)

const businessOverviewTestDriverName = "business-overview-test"

var registerBusinessOverviewTestDriver sync.Once

type businessOverviewTestDriver struct{}

func (businessOverviewTestDriver) Open(string) (driver.Conn, error) {
	return &businessOverviewTestConnection{}, nil
}

type businessOverviewTestConnection struct{}

func (*businessOverviewTestConnection) Prepare(string) (driver.Stmt, error) {
	return nil, driver.ErrSkip
}
func (*businessOverviewTestConnection) Close() error              { return nil }
func (*businessOverviewTestConnection) Begin() (driver.Tx, error) { return nil, driver.ErrSkip }
func (*businessOverviewTestConnection) CheckNamedValue(*driver.NamedValue) error {
	return nil
}

func (*businessOverviewTestConnection) QueryContext(
	_ context.Context,
	query string,
	arguments []driver.NamedValue,
) (driver.Rows, error) {
	if query != businessOverviewPaymentsSQL {
		return nil, fmt.Errorf("unexpected statement: %s", query)
	}
	if len(arguments) < 2 || arguments[0].Value != 20260901 || arguments[1].Value != "ABCN001A002" {
		return nil, fmt.Errorf("unexpected arguments: %#v", arguments)
	}
	return &businessOverviewTestRows{}, nil
}

type businessOverviewTestRows struct{ read bool }

func (*businessOverviewTestRows) Columns() []string {
	return []string{"BILLDATE", "C_STORE_ID", "STORE_NAME", "STORE_CODE", "C_PAYWAY_ID", "PAYAMOUNT", "付款方式"}
}
func (*businessOverviewTestRows) Close() error { return nil }
func (rows *businessOverviewTestRows) Next(values []driver.Value) error {
	if rows.read {
		return io.EOF
	}
	rows.read = true
	copy(values, []driver.Value{int64(20260901), int64(462), " ALLBLU（上海徐汇区徐汇万科广场店） ", "ABCN001A002", int64(24), 3164.76, " 微信 "})
	return nil
}

func TestBusinessOverviewPaymentSQLUsesFixedTableAndBindings(t *testing.T) {
	if !strings.Contains(businessOverviewPaymentsSQL, BusinessOverviewPaymentTable) {
		t.Fatalf("SQL does not use fixed table %s", BusinessOverviewPaymentTable)
	}
	for _, fragment := range []string{"a.BILLDATE = :1", "a.STORE_CODE = :2", "a.PAYAMOUNT", `a.PAYWAY_NAME AS "付款方式"`} {
		if !strings.Contains(businessOverviewPaymentsSQL, fragment) {
			t.Fatalf("SQL is missing %q", fragment)
		}
	}
	if strings.Contains(businessOverviewPaymentsSQL, "%s") || strings.Contains(businessOverviewPaymentsSQL, ";") {
		t.Fatalf("SQL permits dynamic or stacked statements: %s", businessOverviewPaymentsSQL)
	}
}

func TestQueryBusinessOverviewPaymentsBindsAndMapsRows(t *testing.T) {
	registerBusinessOverviewTestDriver.Do(func() {
		sql.Register(businessOverviewTestDriverName, businessOverviewTestDriver{})
	})
	db, err := sql.Open(businessOverviewTestDriverName, "")
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	defer db.Close()

	adapter := &Adapter{db: db, prefetchRows: 100, fetchArraySize: 100}
	items, err := adapter.QueryBusinessOverviewPayments(t.Context(), 20260901, "ABCN001A002")
	if err != nil {
		t.Fatalf("QueryBusinessOverviewPayments() error = %v", err)
	}
	if len(items) != 1 || items[0].BillDate != 20260901 || items[0].StoreID != 462 ||
		items[0].StoreCode != "ABCN001A002" || items[0].PaywayID != 24 || items[0].PayAmount != 3164.76 ||
		items[0].PaywayName != "微信" || items[0].StoreName != "ALLBLU（上海徐汇区徐汇万科广场店）" {
		t.Fatalf("items = %#v", items)
	}
}

func TestQueryBusinessOverviewPaymentsRejectsClosedOrInvalidQuery(t *testing.T) {
	var adapter *Adapter
	if _, err := adapter.QueryBusinessOverviewPayments(t.Context(), 20260901, "ABCN001A002"); err == nil {
		t.Fatal("closed adapter unexpectedly succeeded")
	}
	adapter = &Adapter{db: &sql.DB{}}
	if _, err := adapter.QueryBusinessOverviewPayments(t.Context(), 0, ""); err == nil {
		t.Fatal("invalid filter unexpectedly succeeded")
	}
}
