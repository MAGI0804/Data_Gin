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
	rows       []reportoracle.BojunRetailRow
	maxID      uint64
	queryAfter uint64
	queryCalls int
	closed     int
}

func (connection *fakeBojunOracleConnection) QueryBojunRetailAfterID(_ context.Context, afterID uint64, _ int) ([]reportoracle.BojunRetailRow, error) {
	connection.queryAfter = afterID
	connection.queryCalls++
	return append([]reportoracle.BojunRetailRow(nil), connection.rows...), nil
}

func (*fakeBojunOracleConnection) QueryBojunRetailByModifiedTime(context.Context, time.Time, time.Time, uint64, int) ([]reportoracle.BojunRetailRow, error) {
	return nil, nil
}

func (connection *fakeBojunOracleConnection) MaxBojunRetailID(context.Context) (uint64, error) {
	return connection.maxID, nil
}

func (*fakeBojunOracleConnection) UpdateBojunRetailPushStatus(context.Context, uint64, bool, int) error {
	return nil
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
		retailOrderDAO: &fakeBojunRetailOrderWriter{existing: map[string]bool{}},
		now:            func() time.Time { return time.Date(2026, 8, 26, 10, 0, 0, 0, time.Local) },
		newLeaseToken:  func() string { return "lease-token" },
		batchSize:      100, maxPages: 20, leaseTTL: 2 * time.Minute,
	}
}
