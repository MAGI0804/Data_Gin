package data_svc

import (
	"context"
	"errors"
	"testing"
	"time"

	"gin-biz-web-api/internal/reportoracle"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/model"
)

func TestReportInputQueryDefinitionCRUDValidatesAndPersists(t *testing.T) {
	definitions := &fakeReportInputQueryDefinitionStore{}
	service := NewReportInputQueryServiceWithStores(&fakeReportInputQueryStore{}, definitions, reportInputQueryConfig(), func(context.Context, reportoracle.Config) (reportInputOracleConnection, error) {
		return &fakeReportInputOracle{}, nil
	})
	created, err := service.CreateDefinition(t.Context(), 17, requestbody.ReportInputQueryDefinitionSaveRequest{
		Name: " stores ", SelectSQL: " SELECT id, name FROM stores ", Enabled: true,
	})
	if err != nil || created.ID != 7 || created.Name != "stores" || created.LockVersion != 1 || definitions.created == nil {
		t.Fatalf("CreateDefinition() = %#v, %v store=%#v", created, err, definitions)
	}
	updated, err := service.UpdateDefinition(t.Context(), 17, 7, requestbody.ReportInputQueryDefinitionSaveRequest{
		Name: "stores", SelectSQL: "SELECT id, name FROM current_stores", Enabled: false, ExpectedLockVersion: 1,
	})
	if err != nil || updated.LockVersion != 2 || updated.Enabled || definitions.updated == nil {
		t.Fatalf("UpdateDefinition() = %#v, %v store=%#v", updated, err, definitions)
	}
	deleted, err := service.DeleteDefinition(t.Context(), 17, 7, 2)
	if err != nil || deleted.ID != 7 || definitions.deletedID != 7 {
		t.Fatalf("DeleteDefinition() = %#v, %v deletedID=%d", deleted, err, definitions.deletedID)
	}
}

func TestReportInputQueryDefinitionRejectsUnsafeSelect(t *testing.T) {
	service := NewReportInputQueryServiceWithStores(&fakeReportInputQueryStore{}, &fakeReportInputQueryDefinitionStore{}, reportInputQueryConfig(), func(context.Context, reportoracle.Config) (reportInputOracleConnection, error) {
		return &fakeReportInputOracle{}, nil
	})
	for _, selectSQL := range []string{"DELETE FROM stores", "SELECT id, name FROM stores;", "SELECT id, name FROM stores -- comment"} {
		_, err := service.CreateDefinition(t.Context(), 17, requestbody.ReportInputQueryDefinitionSaveRequest{Name: "stores", SelectSQL: selectSQL, Enabled: true})
		if !errors.Is(err, ErrReportInputQueryInvalid) {
			t.Fatalf("CreateDefinition(%q) error = %v", selectSQL, err)
		}
	}
}

func TestReportInputQueryDefinitionTestUsesSharedOracleAndRecordsSafeStatus(t *testing.T) {
	definitions := &fakeReportInputQueryDefinitionStore{definitions: []model.ReportInputQueryDefinition{{
		BaseModel: model.BaseModel{ID: 7}, Name: "stores", SelectSQL: "SELECT id, name FROM stores", Enabled: true, LockVersion: 1,
	}}}
	oracle := &fakeReportInputOracle{options: []reportoracle.InputOption{{ID: "S001", Name: "上海店"}}}
	openCalls := 0
	service := NewReportInputQueryServiceWithStores(&fakeReportInputQueryStore{}, definitions, reportInputQueryConfig(), func(context.Context, reportoracle.Config) (reportInputOracleConnection, error) {
		openCalls++
		return oracle, nil
	})
	times := []time.Time{time.Unix(100, 0).UTC(), time.Unix(100, int64(25*time.Millisecond)).UTC(), time.Unix(101, 0).UTC(), time.Unix(101, int64(10*time.Millisecond)).UTC()}
	service.now = func() time.Time { value := times[0]; times = times[1:]; return value }
	saved, err := service.TestDefinition(t.Context(), 17, 7, requestbody.ReportInputQueryDefinitionTestRequest{Name: "上海店"})
	if err != nil || saved.Status != "SUCCESS" || saved.RowCount != 1 || saved.LatencyMS != 25 || definitions.testStatus != "SUCCESS" {
		t.Fatalf("TestDefinition() = %#v, %v status=%q", saved, err, definitions.testStatus)
	}
	draft, err := service.TestDefinitionDraft(t.Context(), 17, requestbody.ReportInputQueryDefinitionTestRequest{SelectSQL: "SELECT id, name FROM draft_stores"})
	if err != nil || draft.Status != "SUCCESS" || openCalls != 1 || oracle.statement != "SELECT id, name FROM draft_stores" {
		t.Fatalf("TestDefinitionDraft() = %#v, %v openCalls=%d oracle=%#v", draft, err, openCalls, oracle)
	}
}
