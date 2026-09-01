package data_ctrl

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"gin-biz-web-api/constant"
	"gin-biz-web-api/internal/service/data_svc"

	"github.com/gin-gonic/gin"
)

type fakeBusinessOverviewQueryService struct {
	actor     uint
	afterID   uint
	limit     int
	date      string
	mallCode  string
	calls     int
	listCalls int
}

func (service *fakeBusinessOverviewQueryService) ListMalls(
	_ context.Context,
	actor uint,
	afterID uint,
	limit int,
) (*data_svc.BusinessOverviewMallListResult, error) {
	service.actor, service.afterID, service.limit = actor, afterID, limit
	service.listCalls++
	return &data_svc.BusinessOverviewMallListResult{Items: []data_svc.BusinessOverviewMallDTO{}, NextAfterID: afterID}, nil
}

func (service *fakeBusinessOverviewQueryService) QueryPayments(
	_ context.Context,
	actor uint,
	date string,
	mallCode string,
) (*data_svc.BusinessOverviewPaymentResult, error) {
	service.actor, service.date, service.mallCode = actor, date, mallCode
	service.calls++
	return &data_svc.BusinessOverviewPaymentResult{Date: date, MallCode: mallCode, Items: []data_svc.BusinessOverviewPaymentDTO{}}, nil
}

func TestBusinessOverviewControllerForwardsDateAndMallCode(t *testing.T) {
	service := &fakeBusinessOverviewQueryService{}
	recorder := performBusinessOverviewRequest(t, service, "/api/v1/business-overview/payments?date=20260901&mallCode=ABCN001A002")
	if recorder.Code != http.StatusOK || service.calls != 1 || service.actor != 17 ||
		service.date != "20260901" || service.mallCode != "ABCN001A002" {
		t.Fatalf("status=%d service=%#v body=%s", recorder.Code, service, recorder.Body.String())
	}
}

func TestBusinessOverviewControllerRejectsMissingOrUnknownParameters(t *testing.T) {
	for _, path := range []string{
		"/api/v1/business-overview/payments?date=20260901",
		"/api/v1/business-overview/payments?date=20260901&mallCode=ABCN001A002&extra=1",
		"/api/v1/business-overview/payments?date=20260901&date=20260902&mallCode=ABCN001A002",
	} {
		service := &fakeBusinessOverviewQueryService{}
		recorder := performBusinessOverviewRequest(t, service, path)
		if recorder.Code != http.StatusUnprocessableEntity || service.calls != 0 {
			t.Fatalf("path=%s status=%d calls=%d body=%s", path, recorder.Code, service.calls, recorder.Body.String())
		}
	}
}

func TestBusinessOverviewControllerForwardsMallPagination(t *testing.T) {
	service := &fakeBusinessOverviewQueryService{}
	recorder := performBusinessOverviewRequest(t, service, "/api/v1/business-overview/malls?limit=25&afterId=9")
	if recorder.Code != http.StatusOK || service.listCalls != 1 || service.actor != 17 || service.afterID != 9 || service.limit != 25 {
		t.Fatalf("status=%d service=%#v body=%s", recorder.Code, service, recorder.Body.String())
	}
}

func TestBusinessOverviewControllerRejectsInvalidMallPagination(t *testing.T) {
	for _, path := range []string{
		"/api/v1/business-overview/malls?limit=0",
		"/api/v1/business-overview/malls?limit=201",
		"/api/v1/business-overview/malls?afterId=0",
		"/api/v1/business-overview/malls?afterId=1&extra=1",
		"/api/v1/business-overview/malls?limit=10&limit=20",
	} {
		service := &fakeBusinessOverviewQueryService{}
		recorder := performBusinessOverviewRequest(t, service, path)
		if recorder.Code != http.StatusUnprocessableEntity || service.listCalls != 0 {
			t.Fatalf("path=%s status=%d calls=%d body=%s", path, recorder.Code, service.listCalls, recorder.Body.String())
		}
	}
}

func performBusinessOverviewRequest(
	t *testing.T,
	service BusinessOverviewQueryService,
	path string,
) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(constant.CurrentUserID, "17")
		c.Next()
	})
	controller := NewBusinessOverviewControllerWithService(service)
	router.GET("/api/v1/business-overview/payments", controller.QueryPayments)
	router.GET("/api/v1/business-overview/malls", controller.ListMalls)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	return recorder
}
