package auth_ctrl

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"gin-biz-web-api/internal/service/auth_svc"
	"gin-biz-web-api/pkg/auth"
	"gin-biz-web-api/pkg/responses"

	"github.com/gin-gonic/gin"
)

type AccessMallCatalogService interface {
	ListGrantableMalls(context.Context, uint, uint, int) (*auth_svc.AccessMallQueryResult, error)
}

type AccessMallCatalogController struct {
	service AccessMallCatalogService
}

func NewAccessMallCatalogController() *AccessMallCatalogController {
	return NewAccessMallCatalogControllerWithService(auth_svc.NewAccessAccountService())
}

func NewAccessMallCatalogControllerWithService(service AccessMallCatalogService) *AccessMallCatalogController {
	if service == nil {
		panic("access mall catalog controller: nil service")
	}
	return &AccessMallCatalogController{service: service}
}

func (controller *AccessMallCatalogController) List(c *gin.Context) {
	afterID, limit, ok := parseAccessMallCatalogQuery(c)
	if !ok {
		writeAccessAccountError(c, auth_svc.ErrAccessAccountInvalid)
		return
	}
	result, err := controller.service.ListGrantableMalls(c.Request.Context(), auth.CurrentUserID(c), afterID, limit)
	if err != nil {
		writeAccessAccountError(c, err)
		return
	}
	responses.New(c).ToResponseWithStatus(http.StatusOK, result)
}

func parseAccessMallCatalogQuery(c *gin.Context) (uint, int, bool) {
	query := c.Request.URL.Query()
	if len(query) > 2 || len(query["afterId"]) > 1 || len(query["limit"]) > 1 {
		return 0, 0, false
	}
	for key := range query {
		if key != "afterId" && key != "limit" {
			return 0, 0, false
		}
	}
	limit := 200
	if value, exists := query["limit"]; exists {
		parsed, err := strconv.ParseUint(strings.TrimSpace(value[0]), 10, 16)
		if err != nil || parsed < 1 || parsed > 200 {
			return 0, 0, false
		}
		limit = int(parsed)
	}
	var afterID uint
	if value, exists := query["afterId"]; exists {
		parsed, err := strconv.ParseUint(strings.TrimSpace(value[0]), 10, strconv.IntSize)
		if err != nil || parsed < 1 {
			return 0, 0, false
		}
		afterID = uint(parsed)
	}
	return afterID, limit, true
}
