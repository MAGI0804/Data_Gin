package data_svc

import (
	"context"
	"errors"
	"testing"
	"time"

	appConfig "gin-biz-web-api/config"
	"gin-biz-web-api/internal/reportoracle"
	"gin-biz-web-api/internal/reportrepo"
	"gin-biz-web-api/model"
)

func TestReportInputQueryServiceUsesPublishedBindingAndCachesOracle(t *testing.T) {
	store := &fakeReportInputQueryStore{published: &reportrepo.PublishedReport{Version: model.ReportVersion{
		InputSchemaJSON: model.JSONText(`{"store":{"type":"str","displayName":"门店","control":"SELECT","queryName":"stores"}}`),
	}}}
	connection := &fakeReportInputOracle{options: []reportoracle.InputOption{{ID: "S001", Name: "上海店"}}}
	openCalls := 0
	service := NewReportInputQueryServiceWithDependencies(store, reportInputQueryConfig(), func(context.Context, reportoracle.Config) (reportInputOracleConnection, error) {
		openCalls++
		return connection, nil
	})
	for range 2 {
		result, err := service.Options(t.Context(), 17, 9, "store", "上海店")
		if err != nil || len(result.Items) != 1 || result.Items[0].ID != "S001" {
			t.Fatalf("Options() = %#v, %v", result, err)
		}
	}
	if openCalls != 1 || connection.queryName != "上海店" || connection.statement != "SELECT id, name FROM stores" {
		t.Fatalf("openCalls=%d connection=%#v", openCalls, connection)
	}
}

func TestReportInputQueryServiceEnforcesPublishedReportAccess(t *testing.T) {
	store := &fakeReportInputQueryStore{err: reportrepo.ErrReportActionDenied}
	service := NewReportInputQueryServiceWithDependencies(store, reportInputQueryConfig(), func(context.Context, reportoracle.Config) (reportInputOracleConnection, error) {
		return nil, errors.New("must not open")
	})
	if _, err := service.Options(t.Context(), 17, 9, "store", ""); !errors.Is(err, ErrReportRunDenied) {
		t.Fatalf("Options() error = %v", err)
	}
}

func reportInputQueryConfig() appConfig.ReportInputQueryConfig {
	return appConfig.ReportInputQueryConfig{
		Oracle:  appConfig.ReportInputOracleConfig{Host: "oracle", Port: 1521, ServiceName: "REPORT", Username: "user", Password: "secret", QueryTimeout: time.Second},
		Queries: map[string]appConfig.ReportInputQuery{"stores": {Name: "stores", Select: "SELECT id, name FROM stores"}},
	}
}

type fakeReportInputQueryStore struct {
	published *reportrepo.PublishedReport
	err       error
}

func (store *fakeReportInputQueryStore) FindPublishedReport(context.Context, uint, uint, string) (*reportrepo.PublishedReport, error) {
	return store.published, store.err
}

type fakeReportInputOracle struct {
	options   []reportoracle.InputOption
	statement string
	queryName string
}

func (oracle *fakeReportInputOracle) QueryInputOptions(_ context.Context, statement, name string, _ int) ([]reportoracle.InputOption, error) {
	oracle.statement, oracle.queryName = statement, name
	return oracle.options, nil
}

func (*fakeReportInputOracle) Close() error { return nil }
