package data_svc

import (
	"encoding/json"
	"testing"
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
