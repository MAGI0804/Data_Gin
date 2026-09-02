package auth_ctrl

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"gin-biz-web-api/constant"
	"gin-biz-web-api/internal/service/auth_svc"

	"github.com/gin-gonic/gin"
)

type fakeAccessMallCatalogService struct {
	actor   uint
	afterID uint
	limit   int
	calls   int
}

func (service *fakeAccessMallCatalogService) ListGrantableMalls(
	_ context.Context,
	actor, afterID uint,
	limit int,
) (*auth_svc.AccessMallQueryResult, error) {
	service.actor, service.afterID, service.limit = actor, afterID, limit
	service.calls++
	return &auth_svc.AccessMallQueryResult{Items: []auth_svc.AccessMallDTO{}}, nil
}

func TestAccessMallCatalogControllerListsGrantableMalls(t *testing.T) {
	service := &fakeAccessMallCatalogService{}
	recorder := performAccessMallCatalogRequest(t, service, "/malls?afterId=9&limit=25")
	if recorder.Code != http.StatusOK || service.calls != 1 || service.actor != 17 || service.afterID != 9 || service.limit != 25 {
		t.Fatalf("status=%d service=%#v body=%s", recorder.Code, service, recorder.Body.String())
	}
}

func TestAccessMallCatalogControllerRejectsInvalidPagination(t *testing.T) {
	for _, path := range []string{
		"/malls?limit=0",
		"/malls?limit=201",
		"/malls?afterId=0",
		"/malls?extra=1",
		"/malls?limit=10&limit=20",
	} {
		t.Run(path, func(t *testing.T) {
			service := &fakeAccessMallCatalogService{}
			recorder := performAccessMallCatalogRequest(t, service, path)
			if recorder.Code != http.StatusUnprocessableEntity || service.calls != 0 {
				t.Fatalf("status=%d calls=%d body=%s", recorder.Code, service.calls, recorder.Body.String())
			}
		})
	}
}

func performAccessMallCatalogRequest(
	t *testing.T,
	service AccessMallCatalogService,
	path string,
) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(constant.CurrentUserID, "17")
		c.Next()
	})
	router.GET("/malls", NewAccessMallCatalogControllerWithService(service).List)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	return recorder
}
