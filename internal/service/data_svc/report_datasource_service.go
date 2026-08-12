package data_svc

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	mysqlDriver "github.com/go-sql-driver/mysql"

	"gin-biz-web-api/internal/reportoracle"
	"gin-biz-web-api/internal/reportrepo"
	"gin-biz-web-api/internal/reportsecret"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/model"
)

var (
	ErrReportDatasourceInvalid  = errors.New("report datasource service: invalid input")
	ErrReportDatasourceNotFound = errors.New("report datasource service: not found")
	ErrReportDatasourceConflict = errors.New("report datasource service: conflict")
)

const (
	reportDatasourceTestSuccess = "SUCCESS"
	reportDatasourceTestFailed  = "FAILED"
)

var reportDatasourceCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{2,63}$`)

type reportDatasourceStore interface {
	ListReportDatasources(context.Context) ([]model.ReportDatasource, error)
	GetReportDatasource(context.Context, uint) (*model.ReportDatasource, error)
	CreateReportDatasource(context.Context, uint, *model.ReportDatasource) error
	UpdateReportDatasource(context.Context, uint, *model.ReportDatasource) error
	RecordReportDatasourceTest(context.Context, uint, uint, string, string, time.Time) error
}

type reportDatasourceCipher interface {
	Encrypt(string) (string, string, error)
	Decrypt(string, string) (string, error)
}

type reportDatasourceConnection interface{ Close() error }
type reportDatasourceOpener func(context.Context, reportoracle.Config) (reportDatasourceConnection, error)

type ReportDatasourceService struct {
	store  reportDatasourceStore
	cipher reportDatasourceCipher
	open   reportDatasourceOpener
	now    func() time.Time
}

type ReportDatasourceDTO struct {
	ID                    uint       `json:"id"`
	Code                  string     `json:"code"`
	Name                  string     `json:"name"`
	Driver                string     `json:"driver"`
	Host                  string     `json:"host"`
	Port                  int        `json:"port"`
	ServiceName           string     `json:"serviceName"`
	SID                   string     `json:"sid"`
	Username              string     `json:"username"`
	HasPassword           bool       `json:"hasPassword"`
	SessionTimezone       string     `json:"sessionTimezone"`
	ConnectTimeoutSeconds int        `json:"connectTimeoutSeconds"`
	QueryTimeoutSeconds   int        `json:"queryTimeoutSeconds"`
	MaxOpenConnections    int        `json:"maxOpenConnections"`
	MaxIdleConnections    int        `json:"maxIdleConnections"`
	PrefetchRows          int        `json:"prefetchRows"`
	ArraySize             int        `json:"arraySize"`
	Enabled               bool       `json:"enabled"`
	LastTestStatus        string     `json:"lastTestStatus"`
	LastTestError         string     `json:"lastTestError"`
	LastTestedAt          *time.Time `json:"lastTestedAt"`
}

type ReportDatasourceTestDTO struct {
	Status    string    `json:"status"`
	TestedAt  time.Time `json:"testedAt"`
	LatencyMS int64     `json:"latencyMs"`
	ErrorCode string    `json:"errorCode,omitempty"`
	Message   string    `json:"message"`
}

func NewReportDatasourceService() *ReportDatasourceService {
	return NewReportDatasourceServiceWithDependencies(reportrepo.New(), reportsecret.EnvironmentKeyring{}, func(ctx context.Context, config reportoracle.Config) (reportDatasourceConnection, error) {
		return reportoracle.Open(ctx, config)
	})
}

func NewReportDatasourceServiceWithDependencies(store reportDatasourceStore, cipher reportDatasourceCipher, open reportDatasourceOpener) *ReportDatasourceService {
	if store == nil || cipher == nil || open == nil {
		panic("report datasource service: nil dependency")
	}
	return &ReportDatasourceService{store: store, cipher: cipher, open: open, now: func() time.Time { return time.Now().UTC() }}
}

func (service *ReportDatasourceService) List(ctx context.Context, actor uint) ([]ReportDatasourceDTO, error) {
	if service == nil || ctx == nil || actor == 0 {
		return nil, ErrReportDatasourceInvalid
	}
	items, err := service.store.ListReportDatasources(ctx)
	if err != nil {
		return nil, fmt.Errorf("report datasource service: list: %w", err)
	}
	result := make([]ReportDatasourceDTO, 0, len(items))
	for _, item := range items {
		result = append(result, reportDatasourceDTO(item))
	}
	return result, nil
}

func (service *ReportDatasourceService) Get(ctx context.Context, actor, datasourceID uint) (*ReportDatasourceDTO, error) {
	if service == nil || ctx == nil || actor == 0 || datasourceID == 0 {
		return nil, ErrReportDatasourceInvalid
	}
	datasource, err := service.store.GetReportDatasource(ctx, datasourceID)
	if err != nil {
		return nil, classifyReportDatasourceStoreError(err)
	}
	result := reportDatasourceDTO(*datasource)
	return &result, nil
}

func (service *ReportDatasourceService) Create(ctx context.Context, actor uint, request requestbody.ReportDatasourceSaveRequest) (*ReportDatasourceDTO, error) {
	datasource, err := reportDatasourceFromRequest(request, true)
	if service == nil || ctx == nil || actor == 0 || err != nil {
		return nil, ErrReportDatasourceInvalid
	}
	version, ciphertext, err := service.cipher.Encrypt(request.Password)
	if err != nil {
		return nil, fmt.Errorf("report datasource service: encrypt credential: %w", err)
	}
	datasource.CredentialKeyVersion = version
	datasource.PasswordCiphertext = ciphertext
	if err := service.store.CreateReportDatasource(ctx, actor, datasource); err != nil {
		return nil, classifyReportDatasourceStoreError(err)
	}
	result := reportDatasourceDTO(*datasource)
	return &result, nil
}

func (service *ReportDatasourceService) Update(ctx context.Context, actor, datasourceID uint, request requestbody.ReportDatasourceSaveRequest) (*ReportDatasourceDTO, error) {
	datasource, err := reportDatasourceFromRequest(request, false)
	if service == nil || ctx == nil || actor == 0 || datasourceID == 0 || err != nil {
		return nil, ErrReportDatasourceInvalid
	}
	datasource.ID = datasourceID
	if request.Password != "" {
		version, ciphertext, encryptErr := service.cipher.Encrypt(request.Password)
		if encryptErr != nil {
			return nil, fmt.Errorf("report datasource service: encrypt credential: %w", encryptErr)
		}
		datasource.CredentialKeyVersion = version
		datasource.PasswordCiphertext = ciphertext
	}
	if err := service.store.UpdateReportDatasource(ctx, actor, datasource); err != nil {
		return nil, classifyReportDatasourceStoreError(err)
	}
	persisted, err := service.store.GetReportDatasource(ctx, datasourceID)
	if err != nil {
		return nil, classifyReportDatasourceStoreError(err)
	}
	result := reportDatasourceDTO(*persisted)
	return &result, nil
}

func (service *ReportDatasourceService) Test(ctx context.Context, actor, datasourceID uint) (*ReportDatasourceTestDTO, error) {
	if service == nil || ctx == nil || actor == 0 || datasourceID == 0 {
		return nil, ErrReportDatasourceInvalid
	}
	datasource, err := service.store.GetReportDatasource(ctx, datasourceID)
	if err != nil {
		return nil, classifyReportDatasourceStoreError(err)
	}
	password, err := service.cipher.Decrypt(datasource.CredentialKeyVersion, datasource.PasswordCiphertext)
	if err != nil {
		return nil, fmt.Errorf("report datasource service: decrypt credential: %w", err)
	}
	startedAt := service.now()
	connection, openErr := service.open(ctx, oracleConfigFromDatasource(*datasource, password))
	testedAt := service.now()
	result := &ReportDatasourceTestDTO{Status: reportDatasourceTestSuccess, TestedAt: testedAt, LatencyMS: maxInt64(0, testedAt.Sub(startedAt).Milliseconds()), Message: "Oracle 连接测试成功"}
	if openErr != nil {
		result.Status = reportDatasourceTestFailed
		result.ErrorCode, result.Message = safeDatasourceConnectionFailure(openErr)
	}
	if connection != nil {
		_ = connection.Close()
	}
	safeError := ""
	if result.Status == reportDatasourceTestFailed {
		safeError = result.ErrorCode + ": " + result.Message
	}
	if err := service.store.RecordReportDatasourceTest(ctx, actor, datasourceID, result.Status, safeError, testedAt); err != nil {
		return nil, classifyReportDatasourceStoreError(err)
	}
	return result, nil
}

func reportDatasourceFromRequest(request requestbody.ReportDatasourceSaveRequest, passwordRequired bool) (*model.ReportDatasource, error) {
	request.Code = strings.ToLower(strings.TrimSpace(request.Code))
	request.Name = strings.TrimSpace(request.Name)
	request.Host = strings.TrimSpace(request.Host)
	request.ServiceName = strings.TrimSpace(request.ServiceName)
	request.SID = strings.TrimSpace(request.SID)
	request.Username = strings.TrimSpace(request.Username)
	request.SessionTimezone = strings.TrimSpace(request.SessionTimezone)
	if request.SessionTimezone == "" {
		request.SessionTimezone = "Asia/Shanghai"
	}
	if !reportDatasourceCodePattern.MatchString(request.Code) || request.Name == "" || utf8.RuneCountInString(request.Name) > 128 || request.Host == "" || utf8.RuneCountInString(request.Host) > 255 || request.Username == "" || utf8.RuneCountInString(request.Username) > 128 || request.Port < 1 || request.Port > 65535 || (request.ServiceName == "") == (request.SID == "") || (passwordRequired && request.Password == "") || len(request.Password) > 1024 || request.ConnectTimeoutSeconds < 1 || request.ConnectTimeoutSeconds > 60 || request.QueryTimeoutSeconds < 1 || request.QueryTimeoutSeconds > 86400 || request.MaxOpenConnections < 1 || request.MaxOpenConnections > 100 || request.MaxIdleConnections < 0 || request.MaxIdleConnections > request.MaxOpenConnections || request.PrefetchRows < 1 || request.PrefetchRows > 10000 || request.ArraySize < 1 || request.ArraySize > 10000 {
		return nil, ErrReportDatasourceInvalid
	}
	return &model.ReportDatasource{Code: request.Code, Name: request.Name, Driver: model.ReportDatasourceDriverOracle, Host: request.Host, Port: request.Port, ServiceName: request.ServiceName, SID: request.SID, Username: request.Username, SessionTimezone: request.SessionTimezone, ConnectTimeoutSeconds: request.ConnectTimeoutSeconds, QueryTimeoutSeconds: request.QueryTimeoutSeconds, MaxOpenConnections: request.MaxOpenConnections, MaxIdleConnections: request.MaxIdleConnections, PrefetchRows: request.PrefetchRows, ArraySize: request.ArraySize, Enabled: request.Enabled}, nil
}

func reportDatasourceDTO(datasource model.ReportDatasource) ReportDatasourceDTO {
	return ReportDatasourceDTO{ID: datasource.ID, Code: datasource.Code, Name: datasource.Name, Driver: datasource.Driver, Host: datasource.Host, Port: datasource.Port, ServiceName: datasource.ServiceName, SID: datasource.SID, Username: datasource.Username, HasPassword: datasource.PasswordCiphertext != "", SessionTimezone: datasource.SessionTimezone, ConnectTimeoutSeconds: datasource.ConnectTimeoutSeconds, QueryTimeoutSeconds: datasource.QueryTimeoutSeconds, MaxOpenConnections: datasource.MaxOpenConnections, MaxIdleConnections: datasource.MaxIdleConnections, PrefetchRows: datasource.PrefetchRows, ArraySize: datasource.ArraySize, Enabled: datasource.Enabled, LastTestStatus: datasource.LastTestStatus, LastTestError: datasource.LastTestErrorSafe, LastTestedAt: datasource.LastTestedAt}
}

func safeDatasourceConnectionFailure(err error) (string, string) {
	if errors.Is(err, context.DeadlineExceeded) {
		return "CONNECT_TIMEOUT", "Oracle 连接超时"
	}
	if errors.Is(err, reportoracle.ErrInvalidConfiguration) {
		return "INVALID_CONFIGURATION", "Oracle 连接配置无效"
	}
	upper := strings.ToUpper(err.Error())
	switch {
	case strings.Contains(upper, "ORA-01017"):
		return "AUTHENTICATION_FAILED", "Oracle 用户名或密码无效"
	case strings.Contains(upper, "ORA-12154"), strings.Contains(upper, "ORA-12514"):
		return "SERVICE_NOT_FOUND", "Oracle 服务名或 SID 不可用"
	case strings.Contains(upper, "ORA-12170"), strings.Contains(upper, "ORA-12535"):
		return "CONNECT_TIMEOUT", "Oracle 连接超时"
	case strings.Contains(upper, "ORA-12541"), strings.Contains(upper, "NO SUCH HOST"), strings.Contains(upper, "CONNECTION REFUSED"):
		return "NETWORK_UNREACHABLE", "无法连接 Oracle 网络地址"
	default:
		return "UNKNOWN", "Oracle 连接测试失败"
	}
}

func classifyReportDatasourceStoreError(err error) error {
	var mysqlError *mysqlDriver.MySQLError
	switch {
	case errors.Is(err, reportrepo.ErrDatasourceNotFound):
		return ErrReportDatasourceNotFound
	case errors.Is(err, reportrepo.ErrDatasourceInUse):
		return ErrReportDatasourceConflict
	case errors.As(err, &mysqlError) && mysqlError.Number == 1062:
		return ErrReportDatasourceConflict
	default:
		return fmt.Errorf("report datasource service: store: %w", err)
	}
}

func maxInt64(left, right int64) int64 {
	if left > right {
		return left
	}
	return right
}
