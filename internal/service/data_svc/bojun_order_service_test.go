package data_svc

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/shanghaimall"
)

type fakeBojunRawDataCreator struct {
	created int
	nextID  uint
}

func (f *fakeBojunRawDataCreator) Create(ctx context.Context, rawData *model.RawData) (uint, error) {
	_ = ctx
	f.created++
	if f.nextID == 0 {
		f.nextID = 100
	}
	return f.nextID, nil
}

type fakeBojunRetailOrderWriter struct {
	existing map[string]bool
	created  []string
	failFind bool
}

func (f *fakeBojunRetailOrderWriter) ExistsByDocNo(ctx context.Context, docNo string) (bool, error) {
	_ = ctx
	if f.failFind {
		return false, errors.New("find failed")
	}
	return f.existing[docNo], nil
}

func (f *fakeBojunRetailOrderWriter) CreateIfNotExists(ctx context.Context, order *model.BojunRetailOrder) (bool, error) {
	_ = ctx
	if f.existing[order.DocNo] {
		return false, nil
	}
	f.created = append(f.created, order.DocNo)
	f.existing[order.DocNo] = true
	return true, nil
}

type fakeBojunSyncUpdater struct {
	updates map[uint]int
}

func (f *fakeBojunSyncUpdater) UpdateSyncStatus(ctx context.Context, id uint, synced int) error {
	_ = ctx
	if f.updates == nil {
		f.updates = map[uint]int{}
	}
	f.updates[id] = synced
	return nil
}

type fakeDeliveryLogCreator struct {
	logs []model.DeliveryLog
}

func (f *fakeDeliveryLogCreator) Create(ctx context.Context, log *model.DeliveryLog) (uint, error) {
	_ = ctx
	f.logs = append(f.logs, *log)
	return uint(len(f.logs)), nil
}

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
		defaultBojunOrderMethod,
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

func TestBojunOrderPushSkipsByConfiguredPolicy(t *testing.T) {
	updater := &fakeBojunSyncUpdater{}
	logCreator := &fakeDeliveryLogCreator{}
	service := &BojunOrderPushService{
		retailOrderDAO: updater,
		logDAO:         logCreator,
	}

	result := service.PushNewOrderWithPolicy(context.Background(), &model.BojunRetailOrder{
		BaseModel:      model.BaseModel{ID: 7},
		DocNo:          "B005",
		StoreCode:      "ABCN001P012",
		StoreName:      "前滩",
		TotalQty:       1,
		TotalAmtActual: 99,
	}, 5, OrderPushSkipPolicy{Cycle: 5, Skip: 1})

	if !result.Success || !result.Skipped || result.Error != nil {
		t.Fatalf("result = %+v, want successful skipped push", result)
	}
	if updater.updates[7] != 1 {
		t.Fatalf("sync update = %v, want id 7 synced to 1", updater.updates)
	}
	if len(logCreator.logs) != 1 {
		t.Fatalf("logs length = %d, want 1", len(logCreator.logs))
	}
	if logCreator.logs[0].ResponseBody != "skipped_by_order_push_policy" || !logCreator.logs[0].Success {
		t.Fatalf("log = %+v, want successful policy skip log", logCreator.logs[0])
	}
}

func TestProcessBojunOrderRecordPreviewDoesNotWrite(t *testing.T) {
	rawCreator := &fakeBojunRawDataCreator{}
	orderWriter := &fakeBojunRetailOrderWriter{existing: map[string]bool{}}
	service := &BojunOrderService{
		rawDataDAO:     rawCreator,
		retailOrderDAO: orderWriter,
	}
	result := &BojunOrderSyncResult{}
	record := map[string]interface{}{
		"docno":        "B001",
		"cStoreCode":   "ABCN001P012",
		"cStoreName":   "前滩",
		"totQty":       float64(1),
		"totAmtActual": float64(99.5),
	}

	service.processBojunOrderRecord(context.Background(), record, defaultBojunOrderMethod, "2026-07-08 10:00:00", "2026-07-08 10:30:00", 1, false, result, OrderPushSkipPolicy{})

	if result.TotalCount != 1 || result.PreviewCount != 1 || result.WritableCount != 1 || result.RetailCount != 0 {
		t.Fatalf("result = %+v, want preview without retail write", result)
	}
	if rawCreator.created != 0 || len(orderWriter.created) != 0 {
		t.Fatalf("preview wrote data: raw=%d orders=%v", rawCreator.created, orderWriter.created)
	}
	if len(result.Samples) != 1 || result.Samples[0].Status != "pending" {
		t.Fatalf("samples = %+v, want pending preview sample", result.Samples)
	}
}

func TestProcessBojunOrderRecordConfirmWritesOnlyNewRows(t *testing.T) {
	rawCreator := &fakeBojunRawDataCreator{}
	orderWriter := &fakeBojunRetailOrderWriter{existing: map[string]bool{}}
	service := &BojunOrderService{
		rawDataDAO:     rawCreator,
		retailOrderDAO: orderWriter,
		pushService:    nil,
	}
	result := &BojunOrderSyncResult{}
	record := map[string]interface{}{
		"docno":        "B001",
		"cStoreCode":   "ABCN001P012",
		"cStoreName":   "前滩",
		"totQty":       float64(1),
		"totAmtActual": float64(99.5),
	}

	service.processBojunOrderRecord(context.Background(), record, defaultBojunOrderMethod, "2026-07-08 10:00:00", "2026-07-08 10:30:00", 1, true, result, OrderPushSkipPolicy{})

	if result.TotalCount != 1 || result.SavedCount != 1 || result.RetailCount != 1 || result.FailedCount != 0 {
		t.Fatalf("result = %+v, want saved retail row", result)
	}
	if rawCreator.created != 1 || len(orderWriter.created) != 1 || orderWriter.created[0] != "B001" {
		t.Fatalf("writes raw=%d orders=%v", rawCreator.created, orderWriter.created)
	}
	if len(result.Samples) != 1 || result.Samples[0].Status != "created" {
		t.Fatalf("samples = %+v, want created sample", result.Samples)
	}
}

func TestProcessBojunOrderRecordSkipsExistingRows(t *testing.T) {
	rawCreator := &fakeBojunRawDataCreator{}
	orderWriter := &fakeBojunRetailOrderWriter{existing: map[string]bool{"B001": true}}
	service := &BojunOrderService{
		rawDataDAO:     rawCreator,
		retailOrderDAO: orderWriter,
	}
	result := &BojunOrderSyncResult{}

	service.processBojunOrderRecord(context.Background(), map[string]interface{}{"docno": "B001"}, defaultBojunOrderMethod, "", "", 1, true, result, OrderPushSkipPolicy{})

	if result.SkippedCount != 1 || result.ExistingCount != 1 || result.RetailCount != 0 {
		t.Fatalf("result = %+v, want existing skip", result)
	}
	if rawCreator.created != 0 || len(orderWriter.created) != 0 {
		t.Fatalf("existing row wrote data: raw=%d orders=%v", rawCreator.created, orderWriter.created)
	}
}

func TestProcessBojunOrderRecordInvalidDocNoDoesNotWriteRawData(t *testing.T) {
	rawCreator := &fakeBojunRawDataCreator{}
	orderWriter := &fakeBojunRetailOrderWriter{existing: map[string]bool{}}
	service := &BojunOrderService{
		rawDataDAO:     rawCreator,
		retailOrderDAO: orderWriter,
	}
	result := &BojunOrderSyncResult{}

	service.processBojunOrderRecord(context.Background(), map[string]interface{}{"cStoreCode": "ABCN001P012"}, defaultBojunOrderMethod, "", "", 1, true, result, OrderPushSkipPolicy{})

	if result.FailedCount != 1 || len(result.FailedSamples) != 1 {
		t.Fatalf("result = %+v, want failed sample", result)
	}
	if rawCreator.created != 0 || len(orderWriter.created) != 0 {
		t.Fatalf("invalid row wrote data: raw=%d orders=%v", rawCreator.created, orderWriter.created)
	}
}

func TestNormalizeBojunOrderMethodDefaultsToAsyncEndpoint(t *testing.T) {
	if got := normalizeBojunOrderMethod(""); got != defaultBojunOrderMethod {
		t.Fatalf("method = %s", got)
	}
}

func TestNormalizeBojunOrderMethodUpgradesLegacyEndpoint(t *testing.T) {
	if got := normalizeBojunOrderMethod("/retail/retail.query"); got != defaultBojunOrderMethod {
		t.Fatalf("method = %s", got)
	}
}

func TestNormalizeBojunOrderMethodStripsStandardPrefix(t *testing.T) {
	got := normalizeBojunOrderMethod("/bos/standard/retail/middleretail.query")
	if got != defaultBojunOrderMethod {
		t.Fatalf("method = %s", got)
	}
}

func TestNormalizeBojunOrderTimeRangeAcceptsDatetimeLocal(t *testing.T) {
	start, end, err := normalizeBojunOrderTimeRange("2026-07-08T10:00", "2026-07-08T10:30")
	if err != nil {
		t.Fatalf("normalizeBojunOrderTimeRange returned error: %v", err)
	}
	if start != "2026-07-08 10:00:00" || end != "2026-07-08 10:30:00" {
		t.Fatalf("range = %s/%s", start, end)
	}
}

func TestNormalizeBojunOrderTimeRangeRequiresStartBeforeEnd(t *testing.T) {
	_, _, err := normalizeBojunOrderTimeRange("2026-07-08 10:30:00", "2026-07-08 10:00:00")
	if err == nil {
		t.Fatal("normalizeBojunOrderTimeRange returned nil error, want range error")
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

func TestBojunHangzhouHenglongCodesAreSeparatedFromQimai(t *testing.T) {
	if bojunHangzhouHenglongStoreCode != "416201" {
		t.Fatalf("bojun hangzhou henglong store code = %s, want 416201", bojunHangzhouHenglongStoreCode)
	}
	if bojunHangzhouHenglongItemCode != "E6600000099" {
		t.Fatalf("bojun hangzhou henglong item code = %s, want E6600000099", bojunHangzhouHenglongItemCode)
	}
}
