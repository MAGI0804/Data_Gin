package data_svc

import (
	"context"
	"errors"
	"testing"
	"time"

	appConfig "gin-biz-web-api/config"
	"gin-biz-web-api/internal/reportoracle"
	"gin-biz-web-api/internal/service/auth_svc"
)

type fakeBusinessOverviewOracle struct {
	rows      []reportoracle.BusinessOverviewPaymentRow
	billDate  int
	mallCode  string
	deadline  bool
	queryCall int
}

func (oracle *fakeBusinessOverviewOracle) QueryBusinessOverviewPayments(
	ctx context.Context,
	billDate int,
	mallCode string,
) ([]reportoracle.BusinessOverviewPaymentRow, error) {
	oracle.billDate, oracle.mallCode = billDate, mallCode
	_, oracle.deadline = ctx.Deadline()
	oracle.queryCall++
	return oracle.rows, nil
}

func (*fakeBusinessOverviewOracle) Close() error { return nil }

type fakeBusinessOverviewMallScope struct {
	allowed []string
	err     error
	actor   uint
	codes   []string
}

func (scope *fakeBusinessOverviewMallScope) ConstrainMallCodes(
	_ context.Context,
	actor uint,
	codes []string,
) ([]string, error) {
	scope.actor = actor
	scope.codes = append([]string(nil), codes...)
	return scope.allowed, scope.err
}

func businessOverviewOracleConfig() appConfig.ReportInputOracleConfig {
	return appConfig.ReportInputOracleConfig{
		Host: "oracle", Port: 1521, ServiceName: "REPORT", Username: "user", Password: "secret",
		QueryTimeout: time.Second, PrefetchRows: 100, ArraySize: 100,
	}
}

func TestBusinessOverviewServiceQueriesAuthorizedMallAndCachesOracle(t *testing.T) {
	oracle := &fakeBusinessOverviewOracle{rows: []reportoracle.BusinessOverviewPaymentRow{{
		BillDate: 20260901, StoreID: 462, StoreName: "ALLBLU（上海徐汇区徐汇万科广场店）", StoreCode: "ABCN001A002",
		PaywayID: 24, PayAmount: 3164.76, PaywayName: "微信",
	}}}
	scope := &fakeBusinessOverviewMallScope{allowed: []string{"ABCN001A002"}}
	openCalls := 0
	service := newBusinessOverviewService(businessOverviewOracleConfig(), func(_ context.Context, config reportoracle.Config) (businessOverviewOracle, error) {
		openCalls++
		if config.Host != "oracle" || config.ServiceName != "REPORT" || config.Password != "secret" {
			t.Fatalf("Oracle config = %#v", config)
		}
		return oracle, nil
	}, scope)

	for range 2 {
		result, err := service.QueryPayments(t.Context(), 17, "20260901", "abcn001a002")
		if err != nil {
			t.Fatalf("QueryPayments() error = %v", err)
		}
		if result.Date != "20260901" || result.MallCode != "ABCN001A002" || len(result.Items) != 1 ||
			result.Items[0].PayAmount != 3164.76 || result.Items[0].PaywayName != "微信" {
			t.Fatalf("result = %#v", result)
		}
	}
	if openCalls != 1 || oracle.queryCall != 2 || oracle.billDate != 20260901 || oracle.mallCode != "ABCN001A002" || !oracle.deadline {
		t.Fatalf("openCalls=%d oracle=%#v", openCalls, oracle)
	}
	if scope.actor != 17 || len(scope.codes) != 1 || scope.codes[0] != "ABCN001A002" {
		t.Fatalf("scope = %#v", scope)
	}
}

func TestBusinessOverviewServiceRejectsInvalidDateAndMallCode(t *testing.T) {
	scope := &fakeBusinessOverviewMallScope{allowed: []string{"ABCN001A002"}}
	service := newBusinessOverviewService(businessOverviewOracleConfig(), func(context.Context, reportoracle.Config) (businessOverviewOracle, error) {
		return nil, errors.New("must not open")
	}, scope)
	for _, query := range []struct{ date, mallCode string }{
		{date: "20260230", mallCode: "ABCN001A002"},
		{date: "2026-09-01", mallCode: "ABCN001A002"},
		{date: "20260901", mallCode: "A' OR 1=1--"},
	} {
		if _, err := service.QueryPayments(t.Context(), 17, query.date, query.mallCode); !errors.Is(err, ErrBusinessOverviewInvalid) {
			t.Fatalf("QueryPayments(%q, %q) error = %v", query.date, query.mallCode, err)
		}
	}
}

func TestBusinessOverviewServiceRejectsMallOutsideScope(t *testing.T) {
	service := newBusinessOverviewService(
		businessOverviewOracleConfig(),
		func(context.Context, reportoracle.Config) (businessOverviewOracle, error) {
			return nil, errors.New("must not open")
		},
		&fakeBusinessOverviewMallScope{err: auth_svc.ErrMallScopeForbidden},
	)
	if _, err := service.QueryPayments(t.Context(), 17, "20260901", "ABCN001A002"); !errors.Is(err, ErrBusinessOverviewForbidden) {
		t.Fatalf("QueryPayments() error = %v", err)
	}
}
