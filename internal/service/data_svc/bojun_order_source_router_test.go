package data_svc

import (
	"context"
	"errors"
	"testing"
)

type fakeBojunAPIOrderSyncer struct {
	recentCalls  int
	previewCalls int
	syncCalls    int
}

func (syncer *fakeBojunAPIOrderSyncer) SyncRecentOrders(context.Context) (*BojunOrderSyncResult, error) {
	syncer.recentCalls++
	return &BojunOrderSyncResult{}, nil
}

func (syncer *fakeBojunAPIOrderSyncer) PreviewOrders(context.Context, string, string) (*BojunOrderSyncResult, error) {
	syncer.previewCalls++
	return &BojunOrderSyncResult{}, nil
}

func (syncer *fakeBojunAPIOrderSyncer) SyncOrders(context.Context, string, string) (*BojunOrderSyncResult, error) {
	syncer.syncCalls++
	return &BojunOrderSyncResult{}, nil
}

type fakeBojunOracleOrderSyncer struct {
	incrementalCalls int
	previewCalls     int
	syncCalls        int
}

func (syncer *fakeBojunOracleOrderSyncer) SyncIncremental(context.Context) (*BojunOrderSyncResult, error) {
	syncer.incrementalCalls++
	return &BojunOrderSyncResult{}, nil
}

func (syncer *fakeBojunOracleOrderSyncer) PreviewByModifiedTime(context.Context, string, string) (*BojunOrderSyncResult, error) {
	syncer.previewCalls++
	return &BojunOrderSyncResult{}, nil
}

func (syncer *fakeBojunOracleOrderSyncer) SyncByModifiedTime(context.Context, string, string) (*BojunOrderSyncResult, error) {
	syncer.syncCalls++
	return &BojunOrderSyncResult{}, nil
}

func TestBojunOrderSourceRouterDefaultsToAPI(t *testing.T) {
	api := &fakeBojunAPIOrderSyncer{}
	oracle := &fakeBojunOracleOrderSyncer{}
	router := &BojunOrderSourceRouter{mode: BojunOrderSourceAPI, api: api, oracle: oracle}
	result, err := router.SyncRecentOrders(t.Context())
	if err != nil {
		t.Fatalf("SyncRecentOrders() error = %v", err)
	}
	if api.recentCalls != 1 || oracle.incrementalCalls != 0 || result.SourceMode != "api" {
		t.Fatalf("api calls=%d Oracle calls=%d result=%+v", api.recentCalls, oracle.incrementalCalls, result)
	}
}

func TestBojunOrderSourceRouterRequiresExplicitOracleEnable(t *testing.T) {
	router := &BojunOrderSourceRouter{
		mode: BojunOrderSourceOracle, oracleEnabled: false,
		api: &fakeBojunAPIOrderSyncer{}, oracle: &fakeBojunOracleOrderSyncer{},
	}
	if _, err := router.SyncRecentOrders(t.Context()); !errors.Is(err, ErrBojunOracleSyncDisabled) {
		t.Fatalf("SyncRecentOrders() error = %v", err)
	}
}

func TestBojunOrderSourceRouterUsesOracleModifiedTimeWhenEnabled(t *testing.T) {
	api := &fakeBojunAPIOrderSyncer{}
	oracle := &fakeBojunOracleOrderSyncer{}
	router := &BojunOrderSourceRouter{
		mode: BojunOrderSourceOracle, oracleEnabled: true, api: api, oracle: oracle,
	}
	result, err := router.PreviewOrders(t.Context(), "start", "end")
	if err != nil {
		t.Fatalf("PreviewOrders() error = %v", err)
	}
	if oracle.previewCalls != 1 || api.previewCalls != 0 || result.SourceMode != "oracle" {
		t.Fatalf("api calls=%d Oracle calls=%d result=%+v", api.previewCalls, oracle.previewCalls, result)
	}
}

func TestNormalizeBojunOrderSourceModeRejectsUnknownValue(t *testing.T) {
	if mode, err := NormalizeBojunOrderSourceMode(""); err != nil || mode != BojunOrderSourceAPI {
		t.Fatalf("empty mode = %q, error=%v", mode, err)
	}
	if _, err := NormalizeBojunOrderSourceMode("both"); !errors.Is(err, ErrBojunOrderSourceModeInvalid) {
		t.Fatalf("unknown mode error = %v", err)
	}
}
