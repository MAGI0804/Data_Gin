package data_svc

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	mysqlDriver "github.com/go-sql-driver/mysql"

	appConfig "gin-biz-web-api/config"
	"gin-biz-web-api/internal/reportoracle"
	"gin-biz-web-api/internal/reportrepo"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/model"
)

var (
	ErrReportInputQueryInvalid  = errors.New("report input query service: invalid input")
	ErrReportInputQueryNotFound = errors.New("report input query service: not found")
	ErrReportInputQueryConflict = errors.New("report input query service: conflict")
)

const (
	reportInputQuerySQLMaxBytes = 64 * 1024
	reportInputQueryTestLimit   = 20
)

type ReportInputQueryDefinitionDTO struct {
	ID             uint       `json:"id"`
	Name           string     `json:"name"`
	SelectSQL      string     `json:"selectSql"`
	Enabled        bool       `json:"enabled"`
	LockVersion    uint64     `json:"lockVersion"`
	LastTestStatus string     `json:"lastTestStatus"`
	LastTestError  string     `json:"lastTestError"`
	LastTestedAt   *time.Time `json:"lastTestedAt"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

type ReportInputQueryDefinitionListDTO struct {
	Items []ReportInputQueryDefinitionDTO `json:"items"`
}

type ReportInputQueryDefinitionDeleteDTO struct {
	ID uint `json:"id"`
}

type ReportInputQueryTestDTO struct {
	Status    string                     `json:"status"`
	TestedAt  time.Time                  `json:"testedAt"`
	LatencyMS int64                      `json:"latencyMs"`
	RowCount  int                        `json:"rowCount"`
	Items     []reportoracle.InputOption `json:"items"`
	ErrorCode string                     `json:"errorCode,omitempty"`
	Message   string                     `json:"message"`
}

func (service *ReportInputQueryService) ListDefinitions(ctx context.Context, actor uint) (*ReportInputQueryDefinitionListDTO, error) {
	if service == nil || service.definitions == nil || ctx == nil || actor == 0 {
		return nil, ErrReportInputQueryInvalid
	}
	definitions, err := service.definitions.ListReportInputQueryDefinitions(ctx)
	if err != nil {
		return nil, classifyReportInputQueryStoreError(err)
	}
	result := &ReportInputQueryDefinitionListDTO{Items: make([]ReportInputQueryDefinitionDTO, 0, len(definitions))}
	for _, definition := range definitions {
		result.Items = append(result.Items, reportInputQueryDefinitionDTO(definition))
	}
	return result, nil
}

func (service *ReportInputQueryService) GetDefinition(ctx context.Context, actor, definitionID uint) (*ReportInputQueryDefinitionDTO, error) {
	if service == nil || service.definitions == nil || ctx == nil || actor == 0 || definitionID == 0 {
		return nil, ErrReportInputQueryInvalid
	}
	definition, err := service.definitions.GetReportInputQueryDefinition(ctx, definitionID)
	if err != nil {
		return nil, classifyReportInputQueryStoreError(err)
	}
	result := reportInputQueryDefinitionDTO(*definition)
	return &result, nil
}

func (service *ReportInputQueryService) CreateDefinition(ctx context.Context, actor uint, request requestbody.ReportInputQueryDefinitionSaveRequest) (*ReportInputQueryDefinitionDTO, error) {
	definition, err := reportInputQueryDefinitionFromRequest(request, false)
	if service == nil || service.definitions == nil || ctx == nil || actor == 0 || err != nil {
		return nil, ErrReportInputQueryInvalid
	}
	if err := service.definitions.CreateReportInputQueryDefinition(ctx, actor, definition); err != nil {
		return nil, classifyReportInputQueryStoreError(err)
	}
	result := reportInputQueryDefinitionDTO(*definition)
	return &result, nil
}

func (service *ReportInputQueryService) UpdateDefinition(ctx context.Context, actor, definitionID uint, request requestbody.ReportInputQueryDefinitionSaveRequest) (*ReportInputQueryDefinitionDTO, error) {
	definition, err := reportInputQueryDefinitionFromRequest(request, true)
	if service == nil || service.definitions == nil || ctx == nil || actor == 0 || definitionID == 0 || err != nil {
		return nil, ErrReportInputQueryInvalid
	}
	definition.ID = definitionID
	if err := service.definitions.UpdateReportInputQueryDefinition(ctx, actor, definition, request.ExpectedLockVersion); err != nil {
		return nil, classifyReportInputQueryStoreError(err)
	}
	persisted, err := service.definitions.GetReportInputQueryDefinition(ctx, definitionID)
	if err != nil {
		return nil, classifyReportInputQueryStoreError(err)
	}
	result := reportInputQueryDefinitionDTO(*persisted)
	return &result, nil
}

func (service *ReportInputQueryService) DeleteDefinition(ctx context.Context, actor, definitionID uint, expectedLockVersion uint64) (*ReportInputQueryDefinitionDeleteDTO, error) {
	if service == nil || service.definitions == nil || ctx == nil || actor == 0 || definitionID == 0 || expectedLockVersion == 0 {
		return nil, ErrReportInputQueryInvalid
	}
	if err := service.definitions.DeleteReportInputQueryDefinition(ctx, actor, definitionID, expectedLockVersion); err != nil {
		return nil, classifyReportInputQueryStoreError(err)
	}
	return &ReportInputQueryDefinitionDeleteDTO{ID: definitionID}, nil
}

func (service *ReportInputQueryService) TestDefinition(ctx context.Context, actor, definitionID uint, request requestbody.ReportInputQueryDefinitionTestRequest) (*ReportInputQueryTestDTO, error) {
	if service == nil || service.definitions == nil || ctx == nil || actor == 0 || definitionID == 0 || !validReportInputQueryTestName(request.Name) {
		return nil, ErrReportInputQueryInvalid
	}
	definition, err := service.definitions.GetReportInputQueryDefinition(ctx, definitionID)
	if err != nil {
		return nil, classifyReportInputQueryStoreError(err)
	}
	result := service.testInputQuery(ctx, definition.SelectSQL, request.Name)
	safeError := ""
	if result.Status == reportDatasourceTestFailed {
		safeError = result.ErrorCode + ": " + result.Message
	}
	if err := service.definitions.RecordReportInputQueryTest(ctx, actor, definitionID, result.Status, safeError, result.TestedAt); err != nil {
		return nil, classifyReportInputQueryStoreError(err)
	}
	return result, nil
}

func (service *ReportInputQueryService) TestDefinitionDraft(ctx context.Context, actor uint, request requestbody.ReportInputQueryDefinitionTestRequest) (*ReportInputQueryTestDTO, error) {
	request.SelectSQL = strings.TrimSpace(request.SelectSQL)
	if service == nil || ctx == nil || actor == 0 || !validReportInputQuerySelect(request.SelectSQL) || !validReportInputQueryTestName(request.Name) {
		return nil, ErrReportInputQueryInvalid
	}
	return service.testInputQuery(ctx, request.SelectSQL, request.Name), nil
}

func (service *ReportInputQueryService) testInputQuery(ctx context.Context, selectSQL, exactName string) *ReportInputQueryTestDTO {
	startedAt := service.now()
	queryTimeout := service.config.Oracle.QueryTimeout
	if queryTimeout <= 0 {
		queryTimeout = 30 * time.Second
	}
	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	connection, err := service.connection(queryCtx)
	var items []reportoracle.InputOption
	if err == nil {
		items, err = connection.QueryInputOptions(queryCtx, selectSQL, strings.TrimSpace(exactName), reportInputQueryTestLimit)
	}
	testedAt := service.now()
	result := &ReportInputQueryTestDTO{
		Status: reportDatasourceTestSuccess, TestedAt: testedAt, LatencyMS: maxInt64(0, testedAt.Sub(startedAt).Milliseconds()),
		Items: items, RowCount: len(items), Message: "Oracle 输入查询测试成功",
	}
	if result.Items == nil {
		result.Items = []reportoracle.InputOption{}
	}
	if err != nil {
		result.Status = reportDatasourceTestFailed
		result.ErrorCode, result.Message = safeReportInputQueryFailure(err)
	}
	return result
}

func reportInputQueryDefinitionFromRequest(request requestbody.ReportInputQueryDefinitionSaveRequest, update bool) (*model.ReportInputQueryDefinition, error) {
	name := strings.TrimSpace(request.Name)
	selectSQL := strings.TrimSpace(request.SelectSQL)
	if !appConfig.ValidateReportInputQueryName(name) || !validReportInputQuerySelect(selectSQL) || (update && request.ExpectedLockVersion == 0) || (!update && request.ExpectedLockVersion != 0) {
		return nil, ErrReportInputQueryInvalid
	}
	return &model.ReportInputQueryDefinition{Name: name, SelectSQL: selectSQL, Enabled: request.Enabled}, nil
}

func validReportInputQuerySelect(selectSQL string) bool {
	return selectSQL != "" && len(selectSQL) <= reportInputQuerySQLMaxBytes && appConfig.ValidateReportInputSelect(selectSQL)
}

func validReportInputQueryTestName(name string) bool {
	return utf8.RuneCountInString(strings.TrimSpace(name)) <= 128
}

func reportInputQueryDefinitionDTO(definition model.ReportInputQueryDefinition) ReportInputQueryDefinitionDTO {
	return ReportInputQueryDefinitionDTO{
		ID: definition.ID, Name: definition.Name, SelectSQL: definition.SelectSQL, Enabled: definition.Enabled,
		LockVersion: definition.LockVersion, LastTestStatus: definition.LastTestStatus,
		LastTestError: definition.LastTestErrorSafe, LastTestedAt: definition.LastTestedAt,
		CreatedAt: definition.CreatedAt, UpdatedAt: definition.UpdatedAt,
	}
}

func safeReportInputQueryFailure(err error) (string, string) {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "QUERY_TIMEOUT", "Oracle 输入查询超时"
	case errors.Is(err, reportoracle.ErrMetadataMismatch):
		return "INVALID_COLUMNS", "查询必须且只能返回 id 和 name 两列"
	case errors.Is(err, reportoracle.ErrInvalidConfiguration):
		return "INVALID_QUERY", "查询语句不符合安全约束"
	}
	code, message := safeDatasourceConnectionFailure(err)
	if code == "UNKNOWN" {
		message = "Oracle 输入查询执行失败"
	}
	return code, message
}

func classifyReportInputQueryStoreError(err error) error {
	var mysqlError *mysqlDriver.MySQLError
	switch {
	case errors.Is(err, reportrepo.ErrInputQueryNotFound):
		return ErrReportInputQueryNotFound
	case errors.Is(err, reportrepo.ErrInputQueryVersionConflict), errors.Is(err, reportrepo.ErrInputQueryInUse):
		return ErrReportInputQueryConflict
	case errors.As(err, &mysqlError) && mysqlError.Number == 1062:
		return ErrReportInputQueryConflict
	default:
		return fmt.Errorf("report input query service: store: %w", err)
	}
}
