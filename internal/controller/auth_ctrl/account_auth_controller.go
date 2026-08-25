package auth_ctrl

import (
	"context"
	"errors"
	"strings"

	"github.com/gin-gonic/gin"

	"gin-biz-web-api/internal/requests/auth_request"
	"gin-biz-web-api/internal/service/auth_svc"
	"gin-biz-web-api/pkg/auth"
	"gin-biz-web-api/pkg/errcode"
	"gin-biz-web-api/pkg/logger"
	"gin-biz-web-api/pkg/phonecode"
	"gin-biz-web-api/pkg/responses"
	"gin-biz-web-api/pkg/sms"
	"go.uber.org/zap"
)

type AccountAuthController struct {
	service accountAuthService
}

type accountAuthService interface {
	LoginPassword(ctx context.Context, account, password string) (*auth_svc.ConsoleSessionDTO, error)
	SendPhoneCode(ctx context.Context, phone string, purpose phonecode.Purpose) error
	LoginPhoneCode(ctx context.Context, phone, code string) (*auth_svc.ConsoleSessionDTO, error)
	ResetPassword(ctx context.Context, phone, code, password string) error
	ChangePassword(ctx context.Context, userID uint, currentPassword, newPassword string) error
	Profile(ctx context.Context, userID uint) (*auth_svc.ConsoleProfileDTO, error)
}

func NewAccountAuthController(service accountAuthService) *AccountAuthController {
	return &AccountAuthController{service: service}
}

func NewDatabaseAccountAuthController(codes interface {
	Issue(ctx context.Context, purpose phonecode.Purpose, phoneNumber string) error
	VerifyAndConsume(ctx context.Context, purpose phonecode.Purpose, phoneNumber, code string) error
}) *AccountAuthController {
	return NewAccountAuthController(auth_svc.NewDatabaseAccountAuthService(codes))
}

func (ctrl *AccountAuthController) LoginPassword(c *gin.Context) {
	var request auth_request.PasswordLoginRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		responses.New(c).ToSafeErrorResponse(errcode.BadRequest, "登录参数不正确")
		return
	}
	session, err := ctrl.service.LoginPassword(c.Request.Context(), request.Account, request.Password)
	if err != nil {
		writeAccountAuthError(c, err)
		return
	}
	responses.New(c).ToResponse(session)
}

func (ctrl *AccountAuthController) SendPhoneCode(c *gin.Context) {
	var request auth_request.SendPhoneCodeRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		responses.New(c).ToSafeErrorResponse(errcode.BadRequest, "手机号和验证码用途不能为空")
		return
	}
	purpose := phonecode.Purpose(strings.ToUpper(strings.TrimSpace(request.Purpose)))
	if err := ctrl.service.SendPhoneCode(c.Request.Context(), request.Phone, purpose); err != nil {
		writeAccountAuthError(c, err)
		return
	}
	// The same response is returned whether an eligible account exists.
	responses.New(c).ToResponse(gin.H{"accepted": true})
}

func (ctrl *AccountAuthController) LoginPhoneCode(c *gin.Context) {
	var request auth_request.PhoneCodeLoginRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		responses.New(c).ToSafeErrorResponse(errcode.BadRequest, "手机号或验证码格式不正确")
		return
	}
	session, err := ctrl.service.LoginPhoneCode(c.Request.Context(), request.Phone, request.Code)
	if err != nil {
		writeAccountAuthError(c, err)
		return
	}
	responses.New(c).ToResponse(session)
}

func (ctrl *AccountAuthController) ResetPassword(c *gin.Context) {
	var request auth_request.PasswordResetRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		responses.New(c).ToSafeErrorResponse(errcode.BadRequest, "重置密码参数不正确")
		return
	}
	if err := ctrl.service.ResetPassword(c.Request.Context(), request.Phone, request.Code, request.Password); err != nil {
		writeAccountAuthError(c, err)
		return
	}
	responses.New(c).ToResponse(gin.H{"changed": true})
}

func (ctrl *AccountAuthController) ChangePassword(c *gin.Context) {
	var request auth_request.PasswordChangeRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		responses.New(c).ToSafeErrorResponse(errcode.BadRequest, "修改密码参数不正确")
		return
	}
	if err := ctrl.service.ChangePassword(c.Request.Context(), auth.CurrentUserID(c), request.CurrentPassword, request.NewPassword); err != nil {
		writeAccountAuthError(c, err)
		return
	}
	responses.New(c).ToResponse(gin.H{"changed": true})
}

func (ctrl *AccountAuthController) Profile(c *gin.Context) {
	profile, err := ctrl.service.Profile(c.Request.Context(), auth.CurrentUserID(c))
	if err != nil {
		writeAccountAuthError(c, err)
		return
	}
	responses.New(c).ToResponse(profile)
}

func writeAccountAuthError(c *gin.Context, err error) {
	response := responses.New(c)
	logSMSProviderError(err)
	switch {
	case errors.Is(err, auth_svc.ErrInvalidCredentials), errors.Is(err, auth_svc.ErrAccountUnavailable):
		response.ToSafeErrorResponse(errcode.Unauthorized, "身份凭证无效或账号不可用")
	case errors.Is(err, auth_svc.ErrPasswordTooWeak), errors.Is(err, auth_svc.ErrPasswordUnchanged), errors.Is(err, phonecode.ErrInvalidPhone), errors.Is(err, phonecode.ErrInvalidPurpose):
		response.ToSafeErrorResponse(errcode.UnprocessableEntity, "请求参数不符合要求")
	case errors.Is(err, phonecode.ErrCooldown):
		response.ToSafeErrorResponse(errcode.TooManyRequests, "验证码发送过于频繁")
	default:
		response.ToSafeErrorResponse(errcode.ServiceUnavailable, "认证服务暂时不可用")
	}
}

func logSMSProviderError(err error) {
	if logger.Logger == nil {
		return
	}
	var providerErr *sms.ProviderError
	if !errors.As(err, &providerErr) {
		return
	}
	logger.Warn("短信服务商拒绝发送请求",
		zap.String("provider", "aliyun"),
		zap.String("code", providerErr.Code),
		zap.String("message", providerErr.Message),
		zap.String("request_id", providerErr.RequestID),
	)
}
