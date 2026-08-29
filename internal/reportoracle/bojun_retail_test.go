package reportoracle

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/godror/godror"
)

func TestBojunRetailSQLUsesFixedTableAndBoundFilters(t *testing.T) {
	const expectedTable = "YL_DBS.BJ_REPORT_RETAIL_SF"
	if BojunRetailTable != expectedTable {
		t.Fatalf("BojunRetailTable = %q, want %q", BojunRetailTable, expectedTable)
	}

	for name, statement := range map[string]string{
		"incremental": bojunRetailAfterIDSQL,
		"time range":  bojunRetailModifiedTimeRangeSQL,
		"maximum id":  bojunRetailMaxIDSQL,
		"push status": bojunRetailPushStatusSQL,
	} {
		t.Run(name, func(t *testing.T) {
			if !strings.Contains(statement, BojunRetailTable) {
				t.Fatalf("SQL does not use fixed table %s", BojunRetailTable)
			}
			if strings.Contains(statement, "%s") || strings.Contains(statement, ";") {
				t.Fatalf("SQL permits dynamic or stacked statements: %s", statement)
			}
		})
	}
	for _, fragment := range []string{"M_RETAIL_ID > :1", "ROWNUM <= :2", "ORDER BY M_RETAIL_ID"} {
		if !strings.Contains(bojunRetailAfterIDSQL, fragment) {
			t.Fatalf("incremental SQL is missing %q", fragment)
		}
	}
	for _, fragment := range []string{"MODIFIEDDATE >= :1", "MODIFIEDDATE < :2", "M_RETAIL_ID > :3", "ROWNUM <= :4"} {
		if !strings.Contains(bojunRetailModifiedTimeRangeSQL, fragment) {
			t.Fatalf("time range SQL is missing %q", fragment)
		}
	}
	for _, statement := range []string{bojunRetailAfterIDSQL, bojunRetailModifiedTimeRangeSQL} {
		for _, fragment := range []string{
			"STORE_NAME", "DM_VP_C_VIP_MOBILE", "TOT_AMT_SF", "TOT_AMT_TS", "IS_TOSHOP",
			"NVL(STATUS, '0') AS STATUS", "JSON_ITEM",
		} {
			if !strings.Contains(statement, fragment) {
				t.Fatalf("retail SQL is missing %q", fragment)
			}
		}
	}
}

type bojunRetailScannerFunc func(dest ...interface{}) error

func (scan bojunRetailScannerFunc) Scan(dest ...interface{}) error {
	return scan(dest...)
}

func TestScanBojunRetailRowMapsStoreName(t *testing.T) {
	statusTime := time.Date(2026, 8, 27, 10, 30, 0, 0, time.Local)
	row, err := scanBojunRetailRow(bojunRetailScannerFunc(func(dest ...interface{}) error {
		if len(dest) != 12 {
			return fmt.Errorf("scan destinations = %d, want 12", len(dest))
		}
		*dest[0].(*int64) = 45077
		*dest[1].(*sql.NullString) = sql.NullString{String: " STORE-01 ", Valid: true}
		*dest[2].(*sql.NullString) = sql.NullString{String: " 商场一店 ", Valid: true}
		*dest[3].(*sql.NullString) = sql.NullString{String: " SALE-45077 ", Valid: true}
		*dest[4].(*sql.NullString) = sql.NullString{String: " CMR ", Valid: true}
		*dest[5].(*sql.NullTime) = sql.NullTime{Time: statusTime, Valid: true}
		*dest[6].(*sql.NullString) = sql.NullString{String: "18616613488", Valid: true}
		*dest[7].(*sql.NullFloat64) = sql.NullFloat64{Float64: 88.8, Valid: true}
		*dest[8].(*sql.NullFloat64) = sql.NullFloat64{Float64: 80, Valid: true}
		*dest[9].(*sql.NullString) = sql.NullString{String: "Y", Valid: true}
		*dest[10].(*sql.NullInt64) = sql.NullInt64{Int64: 0, Valid: true}
		*dest[11].(*interface{}) = `[{"no":"SKU-1"}]`
		return nil
	}))
	if err != nil {
		t.Fatalf("scanBojunRetailRow() error = %v", err)
	}
	if row.StoreCode != "STORE-01" || row.StoreName != "商场一店" || row.DocNo != "SALE-45077" {
		t.Fatalf("store/order mapping = %+v", row)
	}
}

func TestBojunRetailBatchSizeValidation(t *testing.T) {
	for _, limit := range []int{1, maxBojunRetailBatchSize} {
		if err := validateBojunRetailBatchSize(limit); err != nil {
			t.Fatalf("validateBojunRetailBatchSize(%d) error = %v", limit, err)
		}
	}
	for _, limit := range []int{0, -1, maxBojunRetailBatchSize + 1} {
		if err := validateBojunRetailBatchSize(limit); err == nil {
			t.Fatalf("validateBojunRetailBatchSize(%d) unexpectedly succeeded", limit)
		}
	}
}

func TestBojunRetailTextSupportsOracleCLOB(t *testing.T) {
	want := `[{"no":"SKU-1"}]`
	got, err := bojunRetailText(godror.Lob{Reader: strings.NewReader(want), IsClob: true})
	if err != nil {
		t.Fatalf("bojunRetailText() error = %v", err)
	}
	if got != want {
		t.Fatalf("bojunRetailText() = %q, want %q", got, want)
	}
}

func TestBojunRetailAdapterRejectsClosedConnection(t *testing.T) {
	var adapter *Adapter
	if _, err := adapter.QueryBojunRetailAfterID(t.Context(), 0, 100); err == nil {
		t.Fatal("QueryBojunRetailAfterID() unexpectedly succeeded")
	}
	if _, err := adapter.MaxBojunRetailID(t.Context()); err == nil {
		t.Fatal("MaxBojunRetailID() unexpectedly succeeded")
	}
	if err := adapter.UpdateBojunRetailPushStatus(t.Context(), 1, true, 20260826); err == nil {
		t.Fatal("UpdateBojunRetailPushStatus() unexpectedly succeeded")
	}
}

const bojunRetailUpdateTestDriverName = "bojun-retail-update-test"

var registerBojunRetailUpdateTestDriver sync.Once

type bojunRetailUpdateTestDriver struct{}

func (bojunRetailUpdateTestDriver) Open(name string) (driver.Conn, error) {
	rowsAffected, err := strconv.ParseInt(name, 10, 64)
	if err != nil {
		return nil, err
	}
	return &bojunRetailUpdateTestConn{rowsAffected: rowsAffected}, nil
}

type bojunRetailUpdateTestConn struct {
	rowsAffected int64
}

func (*bojunRetailUpdateTestConn) Prepare(string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (*bojunRetailUpdateTestConn) Close() error                        { return nil }
func (*bojunRetailUpdateTestConn) Begin() (driver.Tx, error)           { return nil, driver.ErrSkip }

func (connection *bojunRetailUpdateTestConn) ExecContext(
	_ context.Context,
	query string,
	arguments []driver.NamedValue,
) (driver.Result, error) {
	if query != bojunRetailPushStatusSQL {
		return nil, fmt.Errorf("unexpected statement: %s", query)
	}
	if len(arguments) != 3 {
		return nil, fmt.Errorf("update arguments = %d, want 3", len(arguments))
	}
	return driver.RowsAffected(connection.rowsAffected), nil
}

func TestUpdateBojunRetailPushStatusAcceptsAllMatchingRows(t *testing.T) {
	registerBojunRetailUpdateTestDriver.Do(func() {
		sql.Register(bojunRetailUpdateTestDriverName, bojunRetailUpdateTestDriver{})
	})

	tests := []struct {
		name         string
		rowsAffected int64
		wantError    bool
	}{
		{name: "no matching row", rowsAffected: 0, wantError: true},
		{name: "one matching row", rowsAffected: 1},
		{name: "duplicate matching rows", rowsAffected: 7},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db, err := sql.Open(bojunRetailUpdateTestDriverName, strconv.FormatInt(test.rowsAffected, 10))
			if err != nil {
				t.Fatalf("open test database: %v", err)
			}
			defer db.Close()

			adapter := &Adapter{db: db}
			err = adapter.UpdateBojunRetailPushStatus(t.Context(), 46038, true, 20260829)
			if test.wantError {
				if err == nil {
					t.Fatal("UpdateBojunRetailPushStatus() unexpectedly succeeded")
				}
				if !strings.Contains(err.Error(), "retail id 46038 not found") {
					t.Fatalf("UpdateBojunRetailPushStatus() error = %v", err)
				}
			}
			if !test.wantError && err != nil {
				t.Fatalf("UpdateBojunRetailPushStatus() error = %v", err)
			}
		})
	}
}
