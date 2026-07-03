package data_svc

import (
	"encoding/json"
	"testing"

	"gin-biz-web-api/pkg/shanghaimall"
)

func TestBuildBojunOrderRequestBody(t *testing.T) {
	body := buildBojunOrderRequestBody(
		2,
		100,
		"2026-07-03 12:00:00",
		"2026-07-03 12:01:00",
	)

	if body["current"] != 2 {
		t.Fatalf("current = %v", body["current"])
	}
	if body["pageSize"] != 100 {
		t.Fatalf("pageSize = %v", body["pageSize"])
	}
	if body["startTime"] != "2026-07-03 12:00:00" || body["endTime"] != "2026-07-03 12:01:00" {
		t.Fatalf("time range = %v/%v", body["startTime"], body["endTime"])
	}
}

func TestExtractBojunOrderRecords(t *testing.T) {
	payload := map[string]interface{}{
		"code": float64(200),
		"data": map[string]interface{}{
			"current":   float64(1),
			"total":     float64(3),
			"totalPage": float64(2),
			"records": []interface{}{
				map[string]interface{}{"docno": "A001", "totQty": float64(2)},
				map[string]interface{}{"docno": "A002", "totQty": float64(1)},
			},
		},
	}

	records, pageInfo, err := extractBojunOrderRecords(payload)
	if err != nil {
		t.Fatalf("extractBojunOrderRecords returned error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("records length = %d", len(records))
	}
	if pageInfo.Current != 1 || pageInfo.TotalPage != 2 || pageInfo.Total != 3 {
		t.Fatalf("pageInfo = %+v", pageInfo)
	}
}

func TestBuildBojunOrderRawDataMarksSource(t *testing.T) {
	record := map[string]interface{}{
		"docno":      "ABCN001P012P12607031240270004",
		"cStoreCode": "ABCN001P012",
	}

	rawData, err := buildBojunOrderRawData(
		record,
		"/retail/retail.query",
		"2026-07-03 12:00:00",
		"2026-07-03 12:01:00",
		1,
	)
	if err != nil {
		t.Fatalf("buildBojunOrderRawData returned error: %v", err)
	}
	if rawData.Source != bojunOrderSource || rawData.Remark != bojunOrderSource {
		t.Fatalf("source/remark = %s/%s", rawData.Source, rawData.Remark)
	}
	if rawData.ExternalID != "ABCN001P012P12607031240270004" {
		t.Fatalf("external_id = %s", rawData.ExternalID)
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal([]byte(rawData.Metadata), &metadata); err != nil {
		t.Fatal(err)
	}
	if metadata["source"] != bojunOrderSource || metadata["remark"] != bojunOrderSource {
		t.Fatalf("metadata = %v", metadata)
	}

	var rawContent map[string]interface{}
	if err := json.Unmarshal([]byte(rawData.RawContent), &rawContent); err != nil {
		t.Fatal(err)
	}
	if rawContent["docno"] != "ABCN001P012P12607031240270004" {
		t.Fatalf("raw docno = %v", rawContent["docno"])
	}
}

func TestBuildBojunRetailOrderMapsNormalOrder(t *testing.T) {
	record := map[string]interface{}{
		"docno":          "ABCN001P012P12607031240270004",
		"billdate":       float64(20260703),
		"retailbilltype": "CMR",
		"retailsaletype": "CMR",
		"cStoreCode":     "ABCN001P012",
		"cStoreName":     "ALLBLU幼岚（上海浦东新区晶耀前滩店）",
		"totQty":         float64(2),
		"totAmtActual":   float64(446.4),
		"items":          []interface{}{map[string]interface{}{"no": "SKU001"}},
		"payItems":       []interface{}{map[string]interface{}{"cPaywayName": "微信"}},
	}

	order, err := buildBojunRetailOrder(9, record)
	if err != nil {
		t.Fatalf("buildBojunRetailOrder returned error: %v", err)
	}
	if order.RawDataID != 9 || order.DocNo != "ABCN001P012P12607031240270004" {
		t.Fatalf("order key = %d/%s", order.RawDataID, order.DocNo)
	}
	if order.OrderTypeCode != "CMR" || order.OrderTypeName != "正常零售" {
		t.Fatalf("order type = %s/%s", order.OrderTypeCode, order.OrderTypeName)
	}
	if order.RelatedNormalNo != "" {
		t.Fatalf("related normal docno = %s", order.RelatedNormalNo)
	}
	if order.TotalQty != 2 || order.TotalAmtActual != 446.4 {
		t.Fatalf("totals = %d/%v", order.TotalQty, order.TotalAmtActual)
	}
}

func TestBuildBojunRetailOrderMapsRefundOrder(t *testing.T) {
	record := map[string]interface{}{
		"docno":          "ABCN001A004P12606301701270020",
		"description":    "由单据ABCN001A004P12606301638550019退货产生",
		"retailsaletype": "RET",
		"items": []interface{}{
			map[string]interface{}{"orgdocno": "ABCN001A004P12606301638550019", "qty": float64(-1)},
		},
	}

	order, err := buildBojunRetailOrder(10, record)
	if err != nil {
		t.Fatalf("buildBojunRetailOrder returned error: %v", err)
	}
	if order.OrderTypeCode != "RET" || order.OrderTypeName != "退货" {
		t.Fatalf("order type = %s/%s", order.OrderTypeCode, order.OrderTypeName)
	}
	if order.RelatedNormalNo != "ABCN001A004P12606301638550019" {
		t.Fatalf("related normal docno = %s", order.RelatedNormalNo)
	}
}

func TestBuildBojunRetailOrderMapsExchangeOrder(t *testing.T) {
	record := map[string]interface{}{
		"docno":          "ABCN001A001P12607011137100006",
		"description":    "由单据E20260629145733101806231退货产生",
		"retailsaletype": "EXP",
		"items": []interface{}{
			map[string]interface{}{"orgdocno": "E20260629145733101806231", "qty": float64(-1)},
			map[string]interface{}{"qty": float64(1)},
		},
	}

	order, err := buildBojunRetailOrder(11, record)
	if err != nil {
		t.Fatalf("buildBojunRetailOrder returned error: %v", err)
	}
	if order.OrderTypeCode != "EXP" || order.OrderTypeName != "换货" {
		t.Fatalf("order type = %s/%s", order.OrderTypeCode, order.OrderTypeName)
	}
	if order.RelatedNormalNo != "E20260629145733101806231" {
		t.Fatalf("related normal docno = %s", order.RelatedNormalNo)
	}
}

func TestBojunTargetForStoreMapsPushUnits(t *testing.T) {
	cases := map[string]string{
		"ABCN001A001": string(shanghaimall.TargetShangsheng),
		"ABCN001A004": string(shanghaimall.TargetJialiCheng),
		"ABCN001A005": string(shanghaimall.TargetPanlong),
		"ABCN001A003": string(shanghaimall.TargetXintiandi),
		"ABCN001P012": string(shanghaimall.TargetQiantan),
		"ABCN002A001": bojunPushTargetHangzhouHenglong,
	}

	for storeCode, wantTarget := range cases {
		target, ok := bojunTargetForStore(storeCode)
		if !ok {
			t.Fatalf("store %s did not resolve target", storeCode)
		}
		if target.Code != wantTarget {
			t.Fatalf("store %s target = %s, want %s", storeCode, target.Code, wantTarget)
		}
	}
}

func TestBojunTargetForStoreRejectsUnknownStore(t *testing.T) {
	if _, ok := bojunTargetForStore("UNKNOWN"); ok {
		t.Fatal("unknown store unexpectedly resolved target")
	}
}
