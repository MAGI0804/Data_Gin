package data_ctrl

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gin-biz-web-api/internal/service/data_svc"
	"gin-biz-web-api/model"

	"github.com/gin-gonic/gin"
)

type fakeRawRecordListService struct {
	list func(context.Context, data_svc.RawRecordListQuery) (*data_svc.RawRecordListResult, error)
}

func (service fakeRawRecordListService) List(
	ctx context.Context,
	query data_svc.RawRecordListQuery,
) (*data_svc.RawRecordListResult, error) {
	if service.list == nil {
		panic("unexpected raw record list")
	}
	return service.list(ctx, query)
}

func TestRawRecordControllerListUsesValidatedQuery(t *testing.T) {
	service := fakeRawRecordListService{
		list: func(_ context.Context, query data_svc.RawRecordListQuery) (*data_svc.RawRecordListResult, error) {
			if query.Page != 2 || query.PageSize != 30 || query.Source != "api-source" ||
				query.Status != "failed" || query.TraceID != "trace-1" || query.Origin != "receive" {
				t.Fatalf("query = %#v", query)
			}
			if query.StartTime == nil || query.EndTime == nil ||
				query.StartTime.Format(rawRecordDateTimeLayout) != "2026-07-01 00:00:00" ||
				query.EndTime.Format(rawRecordDateTimeLayout) != "2026-07-31 23:59:59" {
				t.Fatalf("time range = %#v / %#v", query.StartTime, query.EndTime)
			}
			return &data_svc.RawRecordListResult{
				List:       []data_svc.RawRecordListItem{},
				Total:      0,
				Page:       query.Page,
				PageSize:   query.PageSize,
				TotalPages: 0,
			}, nil
		},
	}

	recorder := performRawRecordListRequest(t, service, "?page=2&page_size=30&source=api-source&status=failed&trace_id=trace-1&start_time=2026-07-01%2000:00:00&end_time=2026-07-31%2023:59:59&origin=receive")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestRawRecordControllerListDefaultsPagination(t *testing.T) {
	service := fakeRawRecordListService{
		list: func(_ context.Context, query data_svc.RawRecordListQuery) (*data_svc.RawRecordListResult, error) {
			if query.Page != 1 || query.PageSize != 20 {
				t.Fatalf("pagination = %d/%d, want 1/20", query.Page, query.PageSize)
			}
			return &data_svc.RawRecordListResult{
				List:       []data_svc.RawRecordListItem{},
				Page:       1,
				PageSize:   20,
				TotalPages: 0,
			}, nil
		},
	}

	recorder := performRawRecordListRequest(t, service, "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestRawRecordControllerListRejectsInvalidQueryWithoutCallingService(t *testing.T) {
	invalidQueries := []string{
		"?page=0",
		"?page_size=101",
		"?page_size=0",
		"?source=" + strings.Repeat("a", 101),
		"?status=processed",
		"?trace_id=" + strings.Repeat("t", 65),
		"?origin=unknown",
		"?start_time=2026-07-01",
		"?start_time=2026-07-02%2000:00:00&end_time=2026-07-01%2000:00:00",
	}

	for _, query := range invalidQueries {
		t.Run(query, func(t *testing.T) {
			calls := 0
			service := fakeRawRecordListService{
				list: func(context.Context, data_svc.RawRecordListQuery) (*data_svc.RawRecordListResult, error) {
					calls++
					return nil, errors.New("service should not be called")
				},
			}

			recorder := performRawRecordListRequest(t, service, query)
			if recorder.Code != http.StatusBadRequest || calls != 0 {
				t.Fatalf("query=%q status=%d calls=%d body=%s", query, recorder.Code, calls, recorder.Body.String())
			}
		})
	}
}

func TestRawRecordControllerListReturnsOnlySafeFields(t *testing.T) {
	receivedAt := &model.TimeNormal{Time: time.Date(2026, time.July, 1, 12, 30, 0, 0, rawRecordTimeZone)}
	service := fakeRawRecordListService{
		list: func(context.Context, data_svc.RawRecordListQuery) (*data_svc.RawRecordListResult, error) {
			return &data_svc.RawRecordListResult{
				List: []data_svc.RawRecordListItem{{
					ID:         17,
					SourceID:   4,
					SourceCode: "api-source",
					Status:     "queued",
					TraceID:    "trace-safe",
					ReceivedAt: receivedAt,
					CreatedAt:  1782880200,
				}},
				Total:      1,
				Page:       1,
				PageSize:   20,
				TotalPages: 1,
			}, nil
		},
	}

	recorder := performRawRecordListRequest(t, service, "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("data=%#v", body["data"])
	}
	list, ok := data["list"].([]any)
	if !ok || len(list) != 1 {
		t.Fatalf("list=%#v", data["list"])
	}
	item, ok := list[0].(map[string]any)
	if !ok {
		t.Fatalf("item=%#v", list[0])
	}

	for _, field := range []string{"id", "source_id", "source_code", "status", "trace_id", "received_at", "created_at"} {
		if _, ok := item[field]; !ok {
			t.Fatalf("response item does not contain %q: %#v", field, item)
		}
	}
	for _, field := range []string{"raw_content", "headers_json", "query_json", "metadata_json", "error_message", "external_id", "dedupe_hash"} {
		if _, ok := item[field]; ok {
			t.Fatalf("response item unexpectedly contains %q: %#v", field, item)
		}
	}
}

func TestRawRecordControllerListUsesSafeServiceError(t *testing.T) {
	service := fakeRawRecordListService{
		list: func(context.Context, data_svc.RawRecordListQuery) (*data_svc.RawRecordListResult, error) {
			return nil, errors.New("database password=secret")
		},
	}

	recorder := performRawRecordListRequest(t, service, "")
	body := recorder.Body.String()
	if recorder.Code != http.StatusInternalServerError || strings.Contains(body, "password") || strings.Contains(body, "secret") {
		t.Fatalf("status=%d body=%s", recorder.Code, body)
	}
}

func performRawRecordListRequest(
	t *testing.T,
	service RawRecordListService,
	query string,
) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	controller := NewRawRecordControllerWithService(service)
	router.GET("/api/v1/raw-records", controller.List)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/raw-records"+query, nil))
	return recorder
}
