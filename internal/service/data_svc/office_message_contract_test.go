package data_svc

import (
	"database/sql"
	"testing"
	"time"

	"gin-biz-web-api/model"

	"github.com/godror/godror"
)

func TestNormalizeOfficeQueryParametersSupportsYYYYMMDD(t *testing.T) {
	statement := "SELECT ORDER_NO FROM SALES WHERE BILL_DATE = :bill_date AND SHOP = :shop"
	schema, canonical, err := normalizeOfficeQueryParameters(statement, model.JSONText(`[
		{"code":"bill_date","label":"业务日期","valueType":"date","format":"yyyyMMdd","required":true},
		{"code":"shop","label":"店铺","valueType":"string","required":false}
	]`))
	if err != nil || len(schema) != 2 || len(canonical) == 0 {
		t.Fatalf("normalizeOfficeQueryParameters() schema=%#v canonical=%s error=%v", schema, canonical, err)
	}
	stored, arguments, err := normalizeOfficeParameterValues(schema, map[string]string{"bill_date": "20260901", "shop": "S001"})
	if err != nil || string(stored) != `{"bill_date":"20260901","shop":"S001"}` || len(arguments) != 2 {
		t.Fatalf("normalizeOfficeParameterValues() stored=%s arguments=%#v error=%v", stored, arguments, err)
	}
	date, ok := arguments[0].(sql.NamedArg)
	dateValue, dateOK := date.Value.(time.Time)
	if !ok || !dateOK || date.Name != "bill_date" || dateValue.Format("20060102") != "20260901" {
		t.Fatalf("date argument = %#v", arguments[0])
	}
}

func TestNormalizeOfficeQueryParametersIgnoresColonInStringLiteral(t *testing.T) {
	_, _, err := normalizeOfficeQueryParameters(
		"SELECT ':not_a_bind' AS LABEL FROM SALES WHERE BILL_DATE = :bill_date OR NOTE = 'it''s :still_text'",
		model.JSONText(`[{"code":"bill_date","label":"业务日期","valueType":"date","format":"yyyyMMdd","required":true}]`),
	)
	if err != nil {
		t.Fatalf("normalizeOfficeQueryParameters() error = %v", err)
	}
}

func TestNormalizeOfficeQueryParametersSupportsComments(t *testing.T) {
	statement := "\tselect *\n\tfrom BJ_REPORT_RETAIL_DAY_SF a\n\twhere a.billdate=20260901  --单据日期\n\tand a.c_store_id=23        --店仓\n\tand a.c_payway_id=1"
	if _, _, err := normalizeOfficeQueryParameters(statement, model.JSONText(`[]`)); err != nil {
		t.Fatalf("normalizeOfficeQueryParameters() error = %v", err)
	}
	if _, _, err := normalizeOfficeQueryParameters("SELECT * FROM SALES -- :not_a_bind", model.JSONText(`[]`)); err != nil {
		t.Fatalf("normalizeOfficeQueryParameters() treated comment as bind: %v", err)
	}
}

func TestNormalizeOfficeQueryParametersRejectsUnconfiguredBind(t *testing.T) {
	_, _, err := normalizeOfficeQueryParameters(
		"SELECT * FROM SALES WHERE BILL_DATE = :bill_date AND SHOP = :shop",
		model.JSONText(`[{"code":"bill_date","label":"业务日期","valueType":"date","format":"yyyyMMdd","required":true}]`),
	)
	if err == nil {
		t.Fatal("normalizeOfficeQueryParameters() accepted an unconfigured bind")
	}
}

func TestNormalizeOfficeQueryParametersRejectsPositionalBind(t *testing.T) {
	if _, _, err := normalizeOfficeQueryParameters("SELECT * FROM SALES WHERE SHOP = :1", model.JSONText(`[]`)); err == nil {
		t.Fatal("normalizeOfficeQueryParameters() accepted a positional bind")
	}
}

func TestNormalizeOfficeParameterValuesBindsNumbers(t *testing.T) {
	schema := []OfficeQueryParameter{
		{Code: "quantity", Label: "数量", ValueType: "integer", Required: true},
		{Code: "amount", Label: "金额", ValueType: "decimal", Required: true},
	}
	_, arguments, err := normalizeOfficeParameterValues(schema, map[string]string{"quantity": "12", "amount": "19.95"})
	if err != nil {
		t.Fatalf("normalizeOfficeParameterValues() error = %v", err)
	}
	quantity := arguments[0].(sql.NamedArg)
	amount := arguments[1].(sql.NamedArg)
	if quantity.Value != int64(12) || amount.Value != godror.Number("19.95") {
		t.Fatalf("arguments = %#v", arguments)
	}
}

func TestNormalizeOfficeParameterValuesNormalizesInputKeys(t *testing.T) {
	schema := []OfficeQueryParameter{{Code: "shop", Label: "店铺", ValueType: "string", Required: true}}
	stored, arguments, err := normalizeOfficeParameterValues(schema, map[string]string{"SHOP": "S001"})
	if err != nil || string(stored) != `{"shop":"S001"}` || arguments[0].(sql.NamedArg).Value != "S001" {
		t.Fatalf("normalizeOfficeParameterValues() stored=%s arguments=%#v error=%v", stored, arguments, err)
	}
	if _, _, err := normalizeOfficeParameterValues(schema, map[string]string{"shop": "S001", "SHOP": "S002"}); err == nil {
		t.Fatal("normalizeOfficeParameterValues() accepted duplicate case-insensitive keys")
	}
}
