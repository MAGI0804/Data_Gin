package data_svc

import (
	"context"
	"testing"

	"gin-biz-web-api/internal/reportrepo"
	"gin-biz-web-api/model"
)

type fakeReportDownloadCatalogStore struct {
	actor uint
	query reportrepo.DraftListQuery
}

func (store *fakeReportDownloadCatalogStore) ListDrafts(_ context.Context, actor uint, query reportrepo.DraftListQuery) (reportrepo.DraftPage, error) {
	store.actor = actor
	store.query = query
	return reportrepo.DraftPage{Items: []reportrepo.DraftSummary{{
		Definition: model.ReportDefinition{BaseModel: model.BaseModel{ID: 9}, Name: "销售报表", Category: "财务", Status: model.ReportDefinitionStatusActive},
	}}}, nil
}

func TestReportDownloadCatalogServiceRequestsOnlyPublishedExportableReports(t *testing.T) {
	store := &fakeReportDownloadCatalogStore{}
	result, err := NewReportDownloadCatalogServiceWithStore(store).List(t.Context(), 17, 3, 20, " 财务 ", " 销售 ")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if store.actor != 17 || !store.query.PublishedOnly || store.query.Action != reportrepo.ReportActionExport ||
		store.query.AfterID != 3 || store.query.Limit != 20 || store.query.Category != "财务" || store.query.Search != "销售" {
		t.Fatalf("List() query = %#v", store.query)
	}
	if len(result.Items) != 1 || result.Items[0].Status != model.ReportDefinitionStatusActive {
		t.Fatalf("List() = %#v", result)
	}
}
