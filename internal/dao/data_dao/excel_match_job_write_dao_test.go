package data_dao

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestExcelMatchJobDAOBatchUpdateBojunCompletedAtUsesTypedNullOnlyUpdate(t *testing.T) {
	t.Parallel()
	db := dryRunWeatherDAOTestDB(t)
	var statement string
	var statementVars []interface{}
	if err := db.Callback().Raw().After("gorm:raw").Register("test:capture_excel_bojun_completed_at_sql", func(tx *gorm.DB) {
		statement = tx.Statement.SQL.String()
		statementVars = append([]interface{}{}, tx.Statement.Vars...)
	}); err != nil {
		t.Fatalf("register SQL capture callback: %v", err)
	}

	_, err := (&ExcelMatchJobDAO{db: db}).BatchUpdateBojunFieldByKeys(
		t.Context(),
		"docno",
		"completed_at",
		map[string]string{"B001": "2026-07-11 10:31:22"},
	)
	if err != nil {
		t.Fatalf("BatchUpdateBojunFieldByKeys() error=%v", err)
	}
	for _, fragment := range []string{
		"UPDATE bojun_retail_orders",
		"SET `completed_at` = CASE `docno` WHEN ? THEN ? ELSE `completed_at` END",
		"updated_at = ?",
		"WHERE `docno` IN (?)",
		"completed_at IS NULL",
	} {
		if !strings.Contains(statement, fragment) {
			t.Fatalf("statement missing %q: %s", fragment, statement)
		}
	}
	if len(statementVars) != 4 {
		t.Fatalf("vars=%#v", statementVars)
	}
	completedAt, ok := statementVars[1].(time.Time)
	if !ok || completedAt.Format("2006-01-02 15:04:05") != "2026-07-11 10:31:22" ||
		completedAt.Location().String() != "Asia/Shanghai" {
		t.Fatalf("completed_at var=%#v", statementVars[1])
	}
}

func TestExcelMatchJobDAOBatchUpdateBojunCompletedAtRejectsInvalidValueAndMatchField(t *testing.T) {
	t.Parallel()
	dao := &ExcelMatchJobDAO{db: dryRunWeatherDAOTestDB(t)}
	tests := []struct {
		name       string
		matchField string
		value      string
	}{
		{name: "invalid time", matchField: "docno", value: "2026/07/11 10:31:22"},
		{name: "completed at cannot be a match key", matchField: "completed_at", value: "2026-07-11 10:31:22"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := dao.BatchUpdateBojunFieldByKeys(
				t.Context(),
				test.matchField,
				"completed_at",
				map[string]string{"B001": test.value},
			)
			if err == nil {
				t.Fatal("BatchUpdateBojunFieldByKeys() error=nil")
			}
		})
	}
}

func TestBojunExcelImportWriteFieldsUseTypedValuesAndEmptyOnlyConditions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		writeField    string
		value         string
		wantValue     interface{}
		wantCondition string
	}{
		{name: "oracle retail id", writeField: "oracle_retail_id", value: "45077", wantValue: uint64(45077), wantCondition: "oracle_retail_id IS NULL AND is_to_shop IN ('Y', 'N')"},
		{name: "order phone", writeField: "order_phone", value: "18616613488", wantValue: "18616613488", wantCondition: "(order_phone IS NULL OR order_phone = '')"},
		{name: "paid amount", writeField: "paid_amount", value: "470.83", wantValue: "470.83", wantCondition: "paid_amount = 0"},
		{name: "push amount", writeField: "push_amount", value: "80", wantValue: "80", wantCondition: "push_amount = 0"},
		{name: "shop flag", writeField: "is_to_shop", value: "y", wantValue: "Y", wantCondition: "(is_to_shop IS NULL OR is_to_shop = '')"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writeValue, err := bojunExcelWriteValue(tt.writeField, tt.value)
			if err != nil {
				t.Fatalf("bojunExcelWriteValue() error=%v", err)
			}
			if !reflect.DeepEqual(writeValue, tt.wantValue) {
				t.Fatalf("bojunExcelWriteValue()=%#v want=%#v", writeValue, tt.wantValue)
			}
			condition, ok := bojunExcelEmptyWriteCondition(tt.writeField)
			if !ok || condition != tt.wantCondition {
				t.Fatalf("bojunExcelEmptyWriteCondition()=(%q, %t) want=(%q, true)", condition, ok, tt.wantCondition)
			}
		})
	}
}

func TestBojunExcelImportWriteFieldsRejectInvalidValues(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		writeField string
		value      string
	}{
		{name: "oracle retail id zero", writeField: "oracle_retail_id", value: "0"},
		{name: "order phone empty", writeField: "order_phone", value: ""},
		{name: "order phone overlong", writeField: "order_phone", value: strings.Repeat("1", 65)},
		{name: "paid amount text", writeField: "paid_amount", value: "invalid"},
		{name: "paid amount excess scale", writeField: "paid_amount", value: "12.345"},
		{name: "push amount excess precision", writeField: "push_amount", value: "12345678901234567.89"},
		{name: "shop flag unknown", writeField: "is_to_shop", value: "yes"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := bojunExcelWriteValue(tt.writeField, tt.value); err == nil {
				t.Fatal("bojunExcelWriteValue() error=nil")
			}
		})
	}
}

func TestExcelMatchJobDAOBatchUpdateBojunPaidAmountUsesDecimalString(t *testing.T) {
	t.Parallel()
	db := dryRunWeatherDAOTestDB(t)
	var statement string
	var statementVars []interface{}
	if err := db.Callback().Raw().After("gorm:raw").Register("test:capture_excel_bojun_paid_amount_sql", func(tx *gorm.DB) {
		statement = tx.Statement.SQL.String()
		statementVars = append([]interface{}{}, tx.Statement.Vars...)
	}); err != nil {
		t.Fatalf("register SQL capture callback: %v", err)
	}

	_, err := (&ExcelMatchJobDAO{db: db}).BatchUpdateBojunFieldByKeys(
		t.Context(),
		"docno",
		"paid_amount",
		map[string]string{"B001": "470.83"},
	)
	if err != nil {
		t.Fatalf("BatchUpdateBojunFieldByKeys() error=%v", err)
	}
	if !strings.Contains(statement, "paid_amount = 0") {
		t.Fatalf("statement=%s", statement)
	}
	if len(statementVars) != 4 || statementVars[1] != "470.83" {
		t.Fatalf("vars=%#v", statementVars)
	}
}
