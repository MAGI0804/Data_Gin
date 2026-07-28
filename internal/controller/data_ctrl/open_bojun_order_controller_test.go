package data_ctrl

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gin-biz-web-api/constant"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/internal/service/data_svc"

	"github.com/gin-gonic/gin"
)

type fakeOpenBojunOrderQueryService struct {
	request requestbody.OpenBojunOrderQueryRequest
	actor   uint
	calls   int
}

func (service *fakeOpenBojunOrderQueryService) Query(
	_ context.Context,
	actor uint,
	request requestbody.OpenBojunOrderQueryRequest,
) (*data_svc.OpenBojunOrderQueryResult, error) {
	service.actor = actor
	service.request = request
	service.calls++
	return &data_svc.OpenBojunOrderQueryResult{
		Items: []data_svc.OpenBojunOrderDTO{},
		Pagination: data_svc.OpenBojunOrderPagination{
			PageSize: 50,
		},
	}, nil
}

func TestOpenBojunOrderControllerParsesStrictJSON(t *testing.T) {
	service := &fakeOpenBojunOrderQueryService{}
	recorder := performOpenBojunControllerRequest(t, service, `{
		"startDate":"2026-07-01",
		"endDate":"2026-07-31",
		"storeCodes":["ABCN001P012"],
		"pageSize":50
	}`)
	if recorder.Code != http.StatusOK || service.calls != 1 || service.actor != 17 ||
		service.request.StartDate != "2026-07-01" || service.request.PageSize != 50 {
		t.Fatalf("status=%d service=%+v body=%s", recorder.Code, service, recorder.Body.String())
	}
}

func TestOpenBojunOrderControllerRejectsUnknownFields(t *testing.T) {
	service := &fakeOpenBojunOrderQueryService{}
	recorder := performOpenBojunControllerRequest(t, service, `{
		"startDate":"2026-07-01",
		"endDate":"2026-07-31",
		"storeCodes":["ABCN001P012"],
		"mallId":7
	}`)
	if recorder.Code != http.StatusUnprocessableEntity || service.calls != 0 ||
		strings.Contains(recorder.Body.String(), "mallId") {
		t.Fatalf("status=%d calls=%d body=%s", recorder.Code, service.calls, recorder.Body.String())
	}
}

func performOpenBojunControllerRequest(
	t *testing.T,
	service OpenBojunOrderQueryService,
	body string,
) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(constant.CurrentUserID, "17")
		c.Next()
	})
	controller := NewOpenBojunOrderControllerWithService(service)
	router.POST("/api/open/bojun/orders/query", controller.Query)
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/open/bojun/orders/query",
		strings.NewReader(body),
	)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}
