package reportoracle

import (
	"strings"
	"testing"

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
			"DM_VP_C_VIP_MOBILE", "TOT_AMT_SF", "TOT_AMT_TS", "IS_TOSHOP",
			"NVL(STATUS, '0') AS STATUS", "JSON_ITEM",
		} {
			if !strings.Contains(statement, fragment) {
				t.Fatalf("retail SQL is missing %q", fragment)
			}
		}
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
