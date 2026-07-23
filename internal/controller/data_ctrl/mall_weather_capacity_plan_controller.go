package data_ctrl

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"gin-biz-web-api/internal/service/data_svc"
	"gin-biz-web-api/pkg/auth"
	"gin-biz-web-api/pkg/errcode"
	"gin-biz-web-api/pkg/responses"

	"github.com/gin-gonic/gin"
)

type MallWeatherCapacityPlanServiceAPI interface {
	Plan(
		context.Context,
		uint,
		data_svc.MallWeatherCapacityPlanInput,
	) (*data_svc.MallWeatherCapacityPlan, error)
}

type MallWeatherCapacityPlanController struct {
	service MallWeatherCapacityPlanServiceAPI
}

func NewMallWeatherCapacityPlanController() *MallWeatherCapacityPlanController {
	return NewMallWeatherCapacityPlanControllerWithService(data_svc.NewMallWeatherCapacityPlanService())
}

func NewMallWeatherCapacityPlanControllerWithService(
	service MallWeatherCapacityPlanServiceAPI,
) *MallWeatherCapacityPlanController {
	if service == nil {
		panic("mall weather capacity plan controller: nil service")
	}
	return &MallWeatherCapacityPlanController{service: service}
}

func (controller *MallWeatherCapacityPlanController) Show(c *gin.Context) {
	input, err := parseMallWeatherCapacityPlanQuery(c)
	if err != nil {
		writeMallWeatherCapacityPlanError(c, err)
		return
	}
	result, err := controller.service.Plan(c.Request.Context(), auth.CurrentUserID(c), input)
	if err != nil {
		writeMallWeatherCapacityPlanError(c, err)
		return
	}
	responses.New(c).ToResponseWithStatus(http.StatusOK, result)
}

func parseMallWeatherCapacityPlanQuery(c *gin.Context) (data_svc.MallWeatherCapacityPlanInput, error) {
	mallCountValue, err := mallWeatherCapacityPlanQuery(c, "mallCount", "mall_count")
	if err != nil {
		return data_svc.MallWeatherCapacityPlanInput{}, err
	}
	mallCount, err := parseMallWeatherCapacityPlanInt(mallCountValue, "mallCount")
	if err != nil {
		return data_svc.MallWeatherCapacityPlanInput{}, err
	}
	hourlyStepsValue, err := mallWeatherCapacityPlanQuery(c, "hourlySteps", "hourly_steps")
	if err != nil {
		return data_svc.MallWeatherCapacityPlanInput{}, err
	}
	hourlySteps, err := parseMallWeatherCapacityPlanOptionalInt(hourlyStepsValue, "hourlySteps", 360)
	if err != nil {
		return data_svc.MallWeatherCapacityPlanInput{}, err
	}
	dailyStepsValue, err := mallWeatherCapacityPlanQuery(c, "dailySteps", "daily_steps")
	if err != nil {
		return data_svc.MallWeatherCapacityPlanInput{}, err
	}
	dailySteps, err := parseMallWeatherCapacityPlanOptionalInt(dailyStepsValue, "dailySteps", 15)
	if err != nil {
		return data_svc.MallWeatherCapacityPlanInput{}, err
	}
	lifeIndexDaysValue, err := mallWeatherCapacityPlanQuery(c, "lifeIndexDays", "life_index_days")
	if err != nil {
		return data_svc.MallWeatherCapacityPlanInput{}, err
	}
	lifeIndexDays, err := parseMallWeatherCapacityPlanOptionalInt(lifeIndexDaysValue, "lifeIndexDays", 15)
	if err != nil {
		return data_svc.MallWeatherCapacityPlanInput{}, err
	}
	alertsPerMallValue, err := mallWeatherCapacityPlanQuery(c, "alertsPerMall", "alerts_per_mall")
	if err != nil {
		return data_svc.MallWeatherCapacityPlanInput{}, err
	}
	alertsPerMall, err := parseMallWeatherCapacityPlanOptionalInt(alertsPerMallValue, "alertsPerMall", 0)
	if err != nil {
		return data_svc.MallWeatherCapacityPlanInput{}, err
	}
	feishuBatchRowsValue, err := mallWeatherCapacityPlanQuery(c, "feishuBatchRows", "feishu_batch_rows")
	if err != nil {
		return data_svc.MallWeatherCapacityPlanInput{}, err
	}
	feishuBatchRows, err := parseMallWeatherCapacityPlanOptionalInt(feishuBatchRowsValue, "feishuBatchRows", 0)
	if err != nil {
		return data_svc.MallWeatherCapacityPlanInput{}, err
	}
	providerQPSValue, err := mallWeatherCapacityPlanQuery(c, "providerQps", "provider_qps")
	if err != nil {
		return data_svc.MallWeatherCapacityPlanInput{}, err
	}
	providerQPS, err := strconv.ParseFloat(providerQPSValue, 64)
	if err != nil || providerQPSValue == "" {
		return data_svc.MallWeatherCapacityPlanInput{}, fmt.Errorf("%w: invalid providerQps", data_svc.ErrMallWeatherInvalidCapacityPlan)
	}
	return data_svc.MallWeatherCapacityPlanInput{
		MallCount:       mallCount,
		ProviderQPS:     providerQPS,
		HourlySteps:     hourlySteps,
		DailySteps:      dailySteps,
		LifeIndexDays:   lifeIndexDays,
		AlertsPerMall:   alertsPerMall,
		FeishuBatchRows: feishuBatchRows,
	}, nil
}

func mallWeatherCapacityPlanQuery(c *gin.Context, primary string, alias string) (string, error) {
	value, err := weatherAliasedQuery(c, primary, alias)
	if err != nil {
		return "", fmt.Errorf("%w: invalid %s", data_svc.ErrMallWeatherInvalidCapacityPlan, primary)
	}
	return value, nil
}

func parseMallWeatherCapacityPlanInt(value string, field string) (int, error) {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%w: invalid %s", data_svc.ErrMallWeatherInvalidCapacityPlan, field)
	}
	return parsed, nil
}

func parseMallWeatherCapacityPlanOptionalInt(value string, field string, defaultValue int) (int, error) {
	if value == "" {
		return defaultValue, nil
	}
	return parseMallWeatherCapacityPlanInt(value, field)
}

func writeMallWeatherCapacityPlanError(c *gin.Context, err error) {
	code, message := classifyMallWeatherCapacityPlanError(err)
	responses.New(c).ToSafeErrorResponse(code, message)
}

func classifyMallWeatherCapacityPlanError(err error) (*errcode.Error, string) {
	switch {
	case errors.Is(err, data_svc.ErrMallForbidden):
		return errcode.Forbidden, "无权查看天气容量规划"
	case errors.Is(err, data_svc.ErrMallWeatherInvalidCapacityPlan):
		return errcode.UnprocessableEntity, "天气容量规划参数校验失败"
	default:
		return errcode.InternalServerError, "天气容量规划服务暂时不可用"
	}
}
