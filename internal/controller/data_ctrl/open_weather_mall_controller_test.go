package data_ctrl

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"gin-biz-web-api/constant"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/internal/service/data_svc"

	"github.com/gin-gonic/gin"
)

type fakeOpenWeatherMallQueryService struct {
	actor   uint
	request requestbody.OpenWeatherMallQueryRequest
	calls   int
}

func (service *fakeOpenWeatherMallQueryService) Query(
	_ context.Context,
	actor uint,
	request requestbody.OpenWeatherMallQueryRequest,
) (*data_svc.OpenWeatherMallQueryResult, error) {
	service.calls++
	service.actor, service.request = actor, request
	return &data_svc.OpenWeatherMallQueryResult{
		Items:      []data_svc.OpenWeatherMallDTO{},
		Pagination: data_svc.OpenWeatherMallPagination{OpenPagination: data_svc.OpenPagination{PageSize: request.PageSize}},
	}, nil
}

func TestOpenWeatherMallControllerQueryParsesJSON(t *testing.T) {
	service := &fakeOpenWeatherMallQueryService{}
	recorder := performOpenWeatherMallQuery(t, service, `{"cursor":"abc","pageSize":25}`)
	if recorder.Code != http.StatusOK || service.calls != 1 || service.actor != 17 ||
		service.request.Cursor != "abc" || service.request.PageSize != 25 {
		t.Fatalf("status=%d service=%+v body=%s", recorder.Code, service, recorder.Body.String())
	}
}

func TestOpenWeatherMallControllerQueryRejectsUnknownJSONField(t *testing.T) {
	service := &fakeOpenWeatherMallQueryService{}
	recorder := performOpenWeatherMallQuery(t, service, `{"pageSize":25,"unknown":true}`)
	if recorder.Code != http.StatusUnprocessableEntity || service.calls != 0 || strings.Contains(recorder.Body.String(), "unknown") {
		t.Fatalf("status=%d calls=%d body=%s", recorder.Code, service.calls, recorder.Body.String())
	}
}

func performOpenWeatherMallQuery(
	t *testing.T,
	service OpenWeatherMallQueryService,
	body string,
) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(constant.CurrentUserID, strconv.FormatUint(17, 10))
		c.Next()
	})
	controller := NewOpenWeatherMallControllerWithService(service)
	router.POST("/api/open/weather/malls/query", controller.Query)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/open/weather/malls/query",
		strings.NewReader(body),
	)
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	return recorder
}
