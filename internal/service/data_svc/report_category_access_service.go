package data_svc

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"unicode/utf8"

	"gin-biz-web-api/internal/reportrepo"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/model"
)

type reportCategoryAccessStore interface {
	ListCategoryAccess(context.Context, uint) ([]reportrepo.CategoryAccess, error)
	ReplaceCategoryAccess(context.Context, uint, string, uint64, []model.ReportCategoryGrant) (*reportrepo.CategoryAccess, error)
}

type ReportCategoryAccessService struct {
	store reportCategoryAccessStore
}

type ReportCategoryAccessDTO struct {
	Category    string           `json:"category"`
	ReportCount int64            `json:"reportCount"`
	Configured  bool             `json:"configured"`
	LockVersion uint64           `json:"lockVersion"`
	Grants      []ReportGrantDTO `json:"grants"`
}

type ReportCategoryAccessListDTO struct {
	Items []ReportCategoryAccessDTO `json:"items"`
}

func NewReportCategoryAccessService() *ReportCategoryAccessService {
	return NewReportCategoryAccessServiceWithStore(reportrepo.New())
}

func NewReportCategoryAccessServiceWithStore(store reportCategoryAccessStore) *ReportCategoryAccessService {
	if store == nil {
		panic("report category access service: nil store")
	}
	return &ReportCategoryAccessService{store: store}
}

func (service *ReportCategoryAccessService) List(ctx context.Context, actor uint) (*ReportCategoryAccessListDTO, error) {
	if service == nil || service.store == nil || ctx == nil || actor == 0 {
		return nil, invalidReport("service, context and actor are required")
	}
	items, err := service.store.ListCategoryAccess(ctx, actor)
	if err != nil {
		return nil, classifyReportStoreError(err)
	}
	result := &ReportCategoryAccessListDTO{Items: make([]ReportCategoryAccessDTO, 0, len(items))}
	for _, item := range items {
		result.Items = append(result.Items, reportCategoryAccessDTO(item))
	}
	return result, nil
}

func (service *ReportCategoryAccessService) Replace(
	ctx context.Context,
	actor uint,
	request requestbody.ReportCategoryAccessSaveRequest,
) (*ReportCategoryAccessDTO, error) {
	request.Category = strings.TrimSpace(request.Category)
	if service == nil || service.store == nil || ctx == nil || actor == 0 || request.Category == "" || utf8.RuneCountInString(request.Category) > 64 {
		return nil, invalidReport("invalid report category access")
	}
	reportGrants, err := reportGrantsFromRequest(request.Grants, actor)
	if err != nil {
		return nil, err
	}
	for _, grant := range reportGrants {
		var actions []string
		if err := json.Unmarshal([]byte(grant.ActionsJSON), &actions); err != nil {
			return nil, invalidReport("invalid report category access actions")
		}
		hasQuery := false
		hasExport := false
		for _, action := range actions {
			hasQuery = hasQuery || action == reportrepo.ReportActionQuery
			hasExport = hasExport || action == reportrepo.ReportActionExport
		}
		if hasExport && !hasQuery {
			return nil, invalidReport("category export access requires query access")
		}
	}
	grants := make([]model.ReportCategoryGrant, 0, len(reportGrants))
	for _, grant := range reportGrants {
		grants = append(grants, model.ReportCategoryGrant{
			SubjectType: grant.SubjectType,
			SubjectID:   grant.SubjectID,
			ActionsJSON: grant.ActionsJSON,
			CreatedBy:   actor,
			UpdatedBy:   actor,
		})
	}
	saved, err := service.store.ReplaceCategoryAccess(ctx, actor, request.Category, request.ExpectedLockVersion, grants)
	if err != nil {
		switch {
		case errors.Is(err, reportrepo.ErrCategoryAccessNotFound):
			return nil, ErrReportNotFound
		case errors.Is(err, reportrepo.ErrCategoryAccessConflict):
			return nil, ErrReportConflict
		default:
			return nil, classifyReportStoreError(err)
		}
	}
	result := reportCategoryAccessDTO(*saved)
	return &result, nil
}

func reportCategoryAccessDTO(access reportrepo.CategoryAccess) ReportCategoryAccessDTO {
	result := ReportCategoryAccessDTO{
		Category:    access.Policy.Category,
		ReportCount: access.ReportCount,
		Configured:  access.Policy.ID > 0,
		LockVersion: access.Policy.LockVersion,
		Grants:      make([]ReportGrantDTO, 0, len(access.Grants)),
	}
	for _, grant := range access.Grants {
		result.Grants = append(result.Grants, ReportGrantDTO{
			SubjectType: grant.SubjectType,
			SubjectID:   grant.SubjectID,
			Actions:     cloneJSON([]byte(grant.ActionsJSON)),
		})
	}
	return result
}
