package data_ctrl

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/internal/service/data_svc"
	"gin-biz-web-api/pkg/auth"
	"gin-biz-web-api/pkg/errcode"
	"gin-biz-web-api/pkg/responses"

	"github.com/gin-gonic/gin"
)

type MallWeatherExportProfileServiceAPI interface {
	Save(
		context.Context,
		uint,
		requestbody.MallWeatherExportProfileSaveRequest,
	) (*data_svc.MallWeatherExportProfileDTO, bool, error)
	List(
		context.Context,
		uint,
		*bool,
		string,
		int,
	) (*data_svc.MallWeatherExportProfileListResult, error)
}

type MallWeatherExportProfileController struct {
	service MallWeatherExportProfileServiceAPI
}

func NewMallWeatherExportProfileController() *MallWeatherExportProfileController {
	return NewMallWeatherExportProfileControllerWithService(data_svc.NewMallWeatherExportProfileService())
}

func NewMallWeatherExportProfileControllerWithService(
	service MallWeatherExportProfileServiceAPI,
) *MallWeatherExportProfileController {
	if service == nil {
		panic("mall weather export profile controller: nil service")
	}
	return &MallWeatherExportProfileController{service: service}
}

func (controller *MallWeatherExportProfileController) Save(c *gin.Context) {
	var request requestbody.MallWeatherExportProfileSaveRequest
	if err := decodeMallJSON(c, &request); err != nil {
		writeMallWeatherExportProfileError(
			c,
			fmt.Errorf("%w: invalid JSON body", data_svc.ErrMallWeatherExportProfileInvalid),
		)
		return
	}
	result, created, err := controller.service.Save(c.Request.Context(), auth.CurrentUserID(c), request)
	if err != nil {
		writeMallWeatherExportProfileError(c, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	responses.New(c).ToResponseWithStatus(status, result)
}

func (controller *MallWeatherExportProfileController) List(c *gin.Context) {
	enabled, cursor, pageSize, err := parseMallWeatherExportProfileListRequest(c)
	if err != nil {
		writeMallWeatherExportProfileError(c, err)
		return
	}
	result, err := controller.service.List(
		c.Request.Context(),
		auth.CurrentUserID(c),
		enabled,
		cursor,
		pageSize,
	)
	if err != nil {
		writeMallWeatherExportProfileError(c, err)
		return
	}
	responses.New(c).ToResponseWithStatus(http.StatusOK, result)
}

func parseMallWeatherExportProfileListRequest(c *gin.Context) (*bool, string, int, error) {
	var enabled *bool
	if value := strings.TrimSpace(c.Query("enabled")); value != "" {
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return nil, "", 0, fmt.Errorf("%w: invalid enabled", data_svc.ErrMallWeatherExportProfileInvalid)
		}
		enabled = &parsed
	}
	pageSizeValue, err := weatherAliasedQuery(c, "pageSize", "page_size")
	if err != nil {
		return nil, "", 0, fmt.Errorf("%w: invalid pageSize", data_svc.ErrMallWeatherExportProfileInvalid)
	}
	var pageSize int
	if pageSizeValue != "" {
		pageSize, err = strconv.Atoi(pageSizeValue)
		if err != nil || pageSize <= 0 {
			return nil, "", 0, fmt.Errorf("%w: invalid pageSize", data_svc.ErrMallWeatherExportProfileInvalid)
		}
	}
	return enabled, strings.TrimSpace(c.Query("cursor")), pageSize, nil
}

func writeMallWeatherExportProfileError(c *gin.Context, err error) {
	code, message := classifyMallWeatherExportProfileError(err)
	responses.New(c).ToSafeErrorResponse(code, message)
}

func classifyMallWeatherExportProfileError(err error) (*errcode.Error, string) {
	switch {
	case errors.Is(err, data_svc.ErrMallForbidden):
		return errcode.Forbidden, "无权管理天气导出配置"
	case errors.Is(err, data_svc.ErrMallWeatherExportProfileConflict):
		return errcode.Conflict, "天气导出配置版本冲突"
	case errors.Is(err, data_svc.ErrMallWeatherExportProfileInvalid):
		return errcode.UnprocessableEntity, "天气导出配置参数校验失败"
	default:
		return errcode.InternalServerError, "天气导出配置服务暂时不可用"
	}
}
