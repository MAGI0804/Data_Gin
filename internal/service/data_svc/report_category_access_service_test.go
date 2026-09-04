package data_svc

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"gin-biz-web-api/internal/reportrepo"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/model"
)

type fakeReportCategoryAccessStore struct {
	items       []reportrepo.CategoryAccess
	saved       *reportrepo.CategoryAccess
	replaceErr  error
	actor       uint
	category    string
	lockVersion uint64
	grants      []model.ReportCategoryGrant
}

func (store *fakeReportCategoryAccessStore) ListCategoryAccess(context.Context, uint) ([]reportrepo.CategoryAccess, error) {
	return store.items, nil
}

func (store *fakeReportCategoryAccessStore) ReplaceCategoryAccess(
	_ context.Context,
	actor uint,
	category string,
	lockVersion uint64,
	grants []model.ReportCategoryGrant,
) (*reportrepo.CategoryAccess, error) {
	store.actor = actor
	store.category = category
	store.lockVersion = lockVersion
	store.grants = grants
	return store.saved, store.replaceErr
}

func TestReportCategoryAccessServiceListsConfiguredAndUnconfiguredCategories(t *testing.T) {
	store := &fakeReportCategoryAccessStore{items: []reportrepo.CategoryAccess{
		{Policy: model.ReportCategoryAccess{Category: "财务"}, ReportCount: 2, Grants: []model.ReportCategoryGrant{}},
		{
			Policy:      model.ReportCategoryAccess{BaseModel: model.BaseModel{ID: 3}, Category: "运营", LockVersion: 4},
			ReportCount: 5,
			Grants:      []model.ReportCategoryGrant{{SubjectType: "ROLE", SubjectID: 2, ActionsJSON: model.JSONText(`["QUERY","EXPORT"]`)}},
		},
	}}
	result, err := NewReportCategoryAccessServiceWithStore(store).List(t.Context(), 17)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(result.Items) != 2 || result.Items[0].Configured || !result.Items[1].Configured || result.Items[1].LockVersion != 4 || len(result.Items[1].Grants) != 1 {
		t.Fatalf("List() = %#v", result)
	}
}

func TestReportCategoryAccessServiceReplacesTrimmedCategoryPolicy(t *testing.T) {
	store := &fakeReportCategoryAccessStore{saved: &reportrepo.CategoryAccess{
		Policy:      model.ReportCategoryAccess{BaseModel: model.BaseModel{ID: 8}, Category: "财务", LockVersion: 2},
		ReportCount: 3,
		Grants:      []model.ReportCategoryGrant{{SubjectType: "ROLE", SubjectID: 2, ActionsJSON: model.JSONText(`["QUERY","EXPORT"]`)}},
	}}
	request := requestbody.ReportCategoryAccessSaveRequest{
		Category:            " 财务 ",
		ExpectedLockVersion: 1,
		Grants:              []requestbody.ReportGrantRequest{{SubjectType: "role", SubjectID: 2, Actions: json.RawMessage(`["export","query"]`)}},
	}
	result, err := NewReportCategoryAccessServiceWithStore(store).Replace(t.Context(), 17, request)
	if err != nil {
		t.Fatalf("Replace() error = %v", err)
	}
	if store.actor != 17 || store.category != "财务" || store.lockVersion != 1 || len(store.grants) != 1 || result.LockVersion != 2 {
		t.Fatalf("Replace() result=%#v store=%#v", result, store)
	}
	if string(store.grants[0].ActionsJSON) != `["EXPORT","QUERY"]` {
		t.Fatalf("normalized actions = %s", store.grants[0].ActionsJSON)
	}
}

func TestReportCategoryAccessServiceRequiresQueryForExport(t *testing.T) {
	store := &fakeReportCategoryAccessStore{}
	request := requestbody.ReportCategoryAccessSaveRequest{
		Category: "财务",
		Grants:   []requestbody.ReportGrantRequest{{SubjectType: "ROLE", SubjectID: 2, Actions: json.RawMessage(`["EXPORT"]`)}},
	}
	_, err := NewReportCategoryAccessServiceWithStore(store).Replace(t.Context(), 17, request)
	if !errors.Is(err, ErrReportInvalid) || store.category != "" {
		t.Fatalf("Replace() error = %v store=%#v", err, store)
	}
}

func TestReportCategoryAccessServiceMapsOptimisticLockConflict(t *testing.T) {
	store := &fakeReportCategoryAccessStore{replaceErr: reportrepo.ErrCategoryAccessConflict}
	_, err := NewReportCategoryAccessServiceWithStore(store).Replace(t.Context(), 17, requestbody.ReportCategoryAccessSaveRequest{Category: "财务"})
	if !errors.Is(err, ErrReportConflict) {
		t.Fatalf("Replace() error = %v", err)
	}
}
