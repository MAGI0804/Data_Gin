package data_svc

import (
	"context"
	"errors"
	"testing"
	"time"

	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/internal/reportoracle"
	"gin-biz-web-api/model"
)

type fakeBojunOracleDatasourceStore struct {
	datasource model.ReportDatasource
}

func (store *fakeBojunOracleDatasourceStore) FindEnabledReportDatasourceByCode(_ context.Context, code string) (*model.ReportDatasource, error) {
	if code != bojunOracleDatasourceCode {
		return nil, errors.New("unexpected datasource code")
	}
	datasource := store.datasource
	return &datasource, nil
}

type fakeBojunOracleDecryptor struct{}

func (fakeBojunOracleDecryptor) Decrypt(string, string) (string, error) { return "password", nil }

type fakeBojunOracleConnection struct {
	rows          []reportoracle.BojunRetailRow
	maxID         uint64
	queryAfter    uint64
	queryCalls    int
	closed        int
	writeBackIDs  []uint64
	writeBackOK   []bool
	writeBackDate []int
	writeBackErr  error
	modifiedStart time.Time
	modifiedEnd   time.Time
	modifiedAfter uint64
	modifiedCalls int
}

func (connection *fakeBojunOracleConnection) QueryBojunRetailAfterID(_ context.Context, afterID uint64, _ int) ([]reportoracle.BojunRetailRow, error) {
	connection.queryAfter = afterID
	connection.queryCalls++
	return append([]reportoracle.BojunRetailRow(nil), connection.rows...), nil
}

func (connection *fakeBojunOracleConnection) QueryBojunRetailByModifiedTime(_ context.Context, start, end time.Time, afterID uint64, _ int) ([]reportoracle.BojunRetailRow, error) {
	connection.modifiedStart = start
	connection.modifiedEnd = end
	connection.modifiedAfter = afterID
	connection.modifiedCalls++
	return append([]reportoracle.BojunRetailRow(nil), connection.rows...), nil
}

func (connection *fakeBojunOracleConnection) MaxBojunRetailID(context.Context) (uint64, error) {
	return connection.maxID, nil
}

func (connection *fakeBojunOracleConnection) UpdateBojunRetailPushStatus(_ context.Context, retailID uint64, success bool, pushDate int) error {
	connection.writeBackIDs = append(connection.writeBackIDs, retailID)
	connection.writeBackOK = append(connection.writeBackOK, success)
	connection.writeBackDate = append(connection.writeBackDate, pushDate)
	return connection.writeBackErr
}

func (connection *fakeBojunOracleConnection) Close() error {
	connection.closed++
	return nil
}

type fakeBojunOracleStateStore struct {
	state         model.BojunOracleSyncState
	getErr        error
	initializedAt uint64
	leaseAcquired bool
	advancedFrom  uint64
	advancedTo    uint64
	releaseCalls  int
}

type fakeBojunOracleRetailStore struct {
	orders  map[string]*model.BojunRetailOrder
	nextID  uint
	updates map[uint]int
}

func (store *fakeBojunOracleRetailStore) ExistsByDocNo(_ context.Context, docNo string) (bool, error) {
	_, exists := store.orders[docNo]
	return exists, nil
}

func (store *fakeBojunOracleRetailStore) FindByDocNo(_ context.Context, docNo string) (*model.BojunRetailOrder, error) {
	order, exists := store.orders[docNo]
	if !exists {
		return nil, errors.New("order not found")
	}
	copyOrder := *order
	return &copyOrder, nil
}

func (store *fakeBojunOracleRetailStore) CreateIfNotExists(_ context.Context, order *model.BojunRetailOrder) (bool, error) {
	if _, exists := store.orders[order.DocNo]; exists {
		return false, nil
	}
	if store.nextID == 0 {
		store.nextID = 101
	}
	order.ID = store.nextID
	copyOrder := *order
	store.orders[order.DocNo] = &copyOrder
	return true, nil
}

func (store *fakeBojunOracleRetailStore) UpdateSyncStatus(_ context.Context, id uint, synced int) error {
	if store.updates == nil {
		store.updates = make(map[uint]int)
	}
	store.updates[id] = synced
	for _, order := range store.orders {
		if order.ID == id {
			order.Synced = synced
		}
	}
	return nil
}

type fakeBojunOraclePusher struct {
	result bojunOrderPushResult
	calls  int
}

func (pusher *fakeBojunOraclePusher) PushNewOrderWithPolicy(context.Context, *model.BojunRetailOrder, int, OrderPushSkipPolicy) bojunOrderPushResult {
	pusher.calls++
	return pusher.result
}

func (store *fakeBojunOracleStateStore) Get(context.Context, string) (*model.BojunOracleSyncState, error) {
	if store.getErr != nil {
		return nil, store.getErr
	}
	state := store.state
	return &state, nil
}

func (store *fakeBojunOracleStateStore) Initialize(_ context.Context, _ string, retailID uint64, _ time.Time) (*model.BojunOracleSyncState, bool, error) {
	store.initializedAt = retailID
	store.state = model.BojunOracleSyncState{SourceCode: bojunOracleDatasourceCode, LastRetailID: retailID, Initialized: true}
	return &store.state, true, nil
}

func (store *fakeBojunOracleStateStore) AcquireLease(context.Context, string, string, time.Time, time.Duration) (*model.BojunOracleSyncState, bool, error) {
	state := store.state
	return &state, store.leaseAcquired, nil
}

func (store *fakeBojunOracleStateStore) Advance(_ context.Context, _ string, _ string, expected, next uint64, _ time.Time, _ time.Duration) error {
	store.advancedFrom = expected
	store.advancedTo = next
	return nil
}

func (store *fakeBojunOracleStateStore) ReleaseLease(context.Context, string, string, time.Time) error {
	store.releaseCalls++
	return nil
}

func TestBuildBojunRetailOrderFromOracleMapsConfirmedFields(t *testing.T) {
	statusTime := time.Date(2026, 8, 25, 15, 42, 21, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	order, err := buildBojunRetailOrderFromOracle(reportoracle.BojunRetailRow{
		RetailID: 45077, StoreCode: "ABCN001A001", DocNo: "SALE-45077", RetailSaleType: "RET",
		StatusTime: statusTime, OrderPhone: "18616613488", PaidAmount: 470.83, PushAmount: 0,
		IsToShop: "Y", ItemsJSON: "",
	})
	if err != nil {
		t.Fatalf("buildBojunRetailOrderFromOracle() error = %v", err)
	}
	if order.OracleRetailID == nil || *order.OracleRetailID != 45077 || order.CompletedAt == nil || !order.CompletedAt.Equal(statusTime) {
		t.Fatalf("Oracle identity/time mapping = %+v", order)
	}
	if order.OrderPhone != "18616613488" || order.PaidAmount != 470.83 || order.PushAmount != 0 {
		t.Fatalf("phone/amount mapping = %+v", order)
	}
	if order.TotalAmtActual != 470.83 || order.TotalAmtList != 470.83 || order.TotalAmtAcc != 470.83 || order.TotalAmtAcc1 != 470.83 {
		t.Fatalf("legacy paid amount fields = %+v", order)
	}
	if order.OrderTypeCode != "RET" || order.TotalQty != -1 || order.BillDate != 20260825 || order.ItemsJSON != "[]" {
		t.Fatalf("type/date/items mapping = %+v", order)
	}
}

func TestBojunOracleIncrementalInitializesAtCurrentMaximumWithoutFetching(t *testing.T) {
	connection := &fakeBojunOracleConnection{maxID: 45094}
	state := &fakeBojunOracleStateStore{getErr: data_dao.ErrBojunOracleSyncStateNotInitialized}
	service := newTestBojunOracleOrderService(connection, state)

	result, err := service.SyncIncremental(t.Context())
	if err != nil {
		t.Fatalf("SyncIncremental() error = %v", err)
	}
	if !result.WatermarkInitialized || result.WatermarkAfter != 45094 || state.initializedAt != 45094 {
		t.Fatalf("initialization result=%+v state=%+v", result, state)
	}
	if connection.queryCalls != 0 || connection.closed != 1 {
		t.Fatalf("query calls=%d close calls=%d", connection.queryCalls, connection.closed)
	}
}

func TestBojunOracleIncrementalAdvancesAfterPersistingBatch(t *testing.T) {
	statusTime := time.Date(2026, 8, 25, 15, 42, 21, 0, time.Local)
	connection := &fakeBojunOracleConnection{rows: []reportoracle.BojunRetailRow{{
		RetailID: 11, StoreCode: "ABCN001A001", DocNo: "SALE-11", StatusTime: statusTime,
		PaidAmount: 88.8, PushAmount: 80, IsToShop: "N",
	}}}
	state := &fakeBojunOracleStateStore{
		state:         model.BojunOracleSyncState{SourceCode: bojunOracleDatasourceCode, LastRetailID: 10, Initialized: true},
		leaseAcquired: true,
	}
	service := newTestBojunOracleOrderService(connection, state)
	service.batchSize = 2

	result, err := service.SyncIncremental(t.Context())
	if err != nil {
		t.Fatalf("SyncIncremental() error = %v", err)
	}
	if connection.queryAfter != 10 || state.advancedFrom != 10 || state.advancedTo != 11 || state.releaseCalls != 1 {
		t.Fatalf("query/state = after:%d from:%d to:%d releases:%d", connection.queryAfter, state.advancedFrom, state.advancedTo, state.releaseCalls)
	}
	if result.RetailCount != 1 || result.WatermarkAfter != 11 || !result.LeaseAcquired {
		t.Fatalf("result = %+v", result)
	}
}

func TestBojunOracleIncrementalWritesSuccessfulPushBackToOracle(t *testing.T) {
	statusTime := time.Date(2026, 8, 25, 15, 42, 21, 0, time.Local)
	connection := &fakeBojunOracleConnection{rows: []reportoracle.BojunRetailRow{{
		RetailID: 12, StoreCode: "ABCN001A001", DocNo: "SALE-12", StatusTime: statusTime,
		PaidAmount: 88.8, PushAmount: 0, IsToShop: "Y",
	}}}
	state := &fakeBojunOracleStateStore{
		state:         model.BojunOracleSyncState{SourceCode: bojunOracleDatasourceCode, LastRetailID: 11, Initialized: true},
		leaseAcquired: true,
	}
	service := newTestBojunOracleOrderService(connection, state)
	service.batchSize = 2
	pusher := &fakeBojunOraclePusher{result: bojunOrderPushResult{Success: true}}
	service.pushService = pusher
	store := service.retailOrderDAO.(*fakeBojunOracleRetailStore)

	result, err := service.SyncIncremental(t.Context())
	if err != nil {
		t.Fatalf("SyncIncremental() error = %v", err)
	}
	if pusher.calls != 1 || len(connection.writeBackIDs) != 1 || connection.writeBackIDs[0] != 12 || !connection.writeBackOK[0] || connection.writeBackDate[0] != 20260826 {
		t.Fatalf("push/write-back = calls:%d ids:%v status:%v dates:%v", pusher.calls, connection.writeBackIDs, connection.writeBackOK, connection.writeBackDate)
	}
	if result.WatermarkAfter != 12 || store.updates[101] != 1 {
		t.Fatalf("result=%+v local updates=%v", result, store.updates)
	}
}

func TestBojunOracleExistingSuccessfulOrderRetriesOnlyWriteBack(t *testing.T) {
	retailID := uint64(13)
	statusTime := time.Date(2026, 8, 25, 15, 42, 21, 0, time.Local)
	connection := &fakeBojunOracleConnection{rows: []reportoracle.BojunRetailRow{{
		RetailID: retailID, StoreCode: "ABCN001A001", DocNo: "SALE-13", StatusTime: statusTime,
		PaidAmount: 20, PushAmount: 20, IsToShop: "Y", PushStatus: 0,
	}}}
	state := &fakeBojunOracleStateStore{
		state:         model.BojunOracleSyncState{SourceCode: bojunOracleDatasourceCode, LastRetailID: 12, Initialized: true},
		leaseAcquired: true,
	}
	service := newTestBojunOracleOrderService(connection, state)
	service.batchSize = 2
	pusher := &fakeBojunOraclePusher{result: bojunOrderPushResult{Success: true}}
	service.pushService = pusher
	store := service.retailOrderDAO.(*fakeBojunOracleRetailStore)
	store.orders["SALE-13"] = &model.BojunRetailOrder{
		BaseModel: model.BaseModel{ID: 77}, OracleRetailID: &retailID, DocNo: "SALE-13", Synced: 3,
	}

	result, err := service.SyncIncremental(t.Context())
	if err != nil {
		t.Fatalf("SyncIncremental() error = %v", err)
	}
	if pusher.calls != 0 || len(connection.writeBackIDs) != 1 || connection.writeBackIDs[0] != retailID {
		t.Fatalf("pusher calls=%d write-backs=%v", pusher.calls, connection.writeBackIDs)
	}
	if store.updates[77] != 1 || result.WatermarkAfter != retailID {
		t.Fatalf("updates=%v result=%+v", store.updates, result)
	}
}

func TestBojunOracleSuccessfulPushWithFailedWriteBackDoesNotAdvance(t *testing.T) {
	statusTime := time.Date(2026, 8, 25, 15, 42, 21, 0, time.Local)
	connection := &fakeBojunOracleConnection{
		rows: []reportoracle.BojunRetailRow{{
			RetailID: 14, StoreCode: "ABCN001A001", DocNo: "SALE-14", StatusTime: statusTime,
			PaidAmount: 20, PushAmount: 20, IsToShop: "Y",
		}},
		writeBackErr: errors.New("Oracle update failed"),
	}
	state := &fakeBojunOracleStateStore{
		state:         model.BojunOracleSyncState{SourceCode: bojunOracleDatasourceCode, LastRetailID: 13, Initialized: true},
		leaseAcquired: true,
	}
	service := newTestBojunOracleOrderService(connection, state)
	service.batchSize = 2
	service.pushService = &fakeBojunOraclePusher{result: bojunOrderPushResult{Success: true}}
	store := service.retailOrderDAO.(*fakeBojunOracleRetailStore)

	result, err := service.SyncIncremental(t.Context())
	if err == nil {
		t.Fatal("SyncIncremental() unexpectedly succeeded")
	}
	if state.advancedTo != 0 || result.WatermarkAfter != 13 {
		t.Fatalf("watermark advanced after failed write-back: state=%+v result=%+v", state, result)
	}
	if store.updates[101] != 3 {
		t.Fatalf("local sync status = %d, want pending write-back 3", store.updates[101])
	}
}

func TestBojunOraclePreviewQueriesModifiedTimeWithoutWritingOrAdvancing(t *testing.T) {
	statusTime := time.Date(2026, 8, 25, 15, 42, 21, 0, time.Local)
	connection := &fakeBojunOracleConnection{rows: []reportoracle.BojunRetailRow{{
		RetailID: 15, StoreCode: "ABCN001A001", DocNo: "SALE-15", StatusTime: statusTime,
		PaidAmount: 20, PushAmount: 20, IsToShop: "Y",
	}}}
	state := &fakeBojunOracleStateStore{}
	service := newTestBojunOracleOrderService(connection, state)
	service.batchSize = 2
	rawStore := service.rawDataDAO.(*fakeBojunRawDataCreator)

	result, err := service.PreviewByModifiedTime(t.Context(), "2026-08-25T10:00", "2026-08-25T11:00")
	if err != nil {
		t.Fatalf("PreviewByModifiedTime() error = %v", err)
	}
	if connection.modifiedCalls != 1 || connection.modifiedStart.Hour() != 10 || connection.modifiedEnd.Hour() != 11 || connection.modifiedAfter != 0 {
		t.Fatalf("modified query start=%v end=%v after=%d calls=%d", connection.modifiedStart, connection.modifiedEnd, connection.modifiedAfter, connection.modifiedCalls)
	}
	if result.PreviewCount != 1 || result.WritableCount != 1 || rawStore.created != 0 || state.advancedTo != 0 {
		t.Fatalf("result=%+v raw writes=%d state=%+v", result, rawStore.created, state)
	}
}

func newTestBojunOracleOrderService(connection bojunOracleConnection, state bojunOracleSyncStateStore) *BojunOracleOrderService {
	return &BojunOracleOrderService{
		datasourceStore: &fakeBojunOracleDatasourceStore{datasource: model.ReportDatasource{
			Code: bojunOracleDatasourceCode, Driver: model.ReportDatasourceDriverOracle, Enabled: true,
			CredentialKeyVersion: "v1", PasswordCiphertext: "ciphertext", QueryTimeoutSeconds: 1,
		}},
		decryptor:      fakeBojunOracleDecryptor{},
		openOracle:     func(context.Context, reportoracle.Config) (bojunOracleConnection, error) { return connection, nil },
		stateStore:     state,
		rawDataDAO:     &fakeBojunRawDataCreator{nextID: 100},
		retailOrderDAO: &fakeBojunOracleRetailStore{orders: map[string]*model.BojunRetailOrder{}},
		now:            func() time.Time { return time.Date(2026, 8, 26, 10, 0, 0, 0, time.Local) },
		newLeaseToken:  func() string { return "lease-token" },
		batchSize:      100, maxPages: 20, leaseTTL: 2 * time.Minute,
	}
}
