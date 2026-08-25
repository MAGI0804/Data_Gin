package data_svc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"

	appConfig "gin-biz-web-api/config"
	"gin-biz-web-api/internal/reportoracle"
	"gin-biz-web-api/internal/reportrepo"
)

var ErrReportInputQueryUnavailable = errors.New("report input query service: unavailable")

const reportInputOptionLimit = 100

type reportInputQueryStore interface {
	FindPublishedReport(context.Context, uint, uint, string) (*reportrepo.PublishedReport, error)
}

type reportInputOracleConnection interface {
	QueryInputOptions(context.Context, string, string, int) ([]reportoracle.InputOption, error)
	Close() error
}

type reportInputOracleOpener func(context.Context, reportoracle.Config) (reportInputOracleConnection, error)

type ReportInputQueryService struct {
	store  reportInputQueryStore
	config appConfig.ReportInputQueryConfig
	open   reportInputOracleOpener
	mu     sync.Mutex
	oracle reportInputOracleConnection
}

type ReportInputQueryListDTO struct {
	Items []string `json:"items"`
}

type ReportInputOptionListDTO struct {
	Items []reportoracle.InputOption `json:"items"`
}

func NewReportInputQueryService() (*ReportInputQueryService, error) {
	configured, err := appConfig.LoadReportInputQueryConfig()
	if err != nil {
		return nil, err
	}
	return NewReportInputQueryServiceWithDependencies(reportrepo.New(), configured, func(ctx context.Context, config reportoracle.Config) (reportInputOracleConnection, error) {
		return reportoracle.Open(ctx, config)
	}), nil
}

func NewReportInputQueryServiceWithDependencies(store reportInputQueryStore, configured appConfig.ReportInputQueryConfig, opener reportInputOracleOpener) *ReportInputQueryService {
	if store == nil || opener == nil {
		panic("report input query service: dependencies are required")
	}
	if configured.Queries == nil {
		configured.Queries = map[string]appConfig.ReportInputQuery{}
	}
	return &ReportInputQueryService{store: store, config: configured, open: opener}
}

func (service *ReportInputQueryService) List() *ReportInputQueryListDTO {
	names := make([]string, 0, len(service.config.Queries))
	for name := range service.config.Queries {
		names = append(names, name)
	}
	sort.Strings(names)
	return &ReportInputQueryListDTO{Items: names}
}

func (service *ReportInputQueryService) Options(ctx context.Context, actor, definitionID uint, conditionCode, exactName string) (*ReportInputOptionListDTO, error) {
	conditionCode = strings.TrimSpace(conditionCode)
	exactName = strings.TrimSpace(exactName)
	if service == nil || service.store == nil || ctx == nil || actor == 0 || definitionID == 0 ||
		!reportLogicalCodePattern.MatchString(conditionCode) || utf8.RuneCountInString(exactName) > 128 {
		return nil, fmt.Errorf("%w: invalid report input option request", ErrReportRunInvalid)
	}
	published, err := service.store.FindPublishedReport(ctx, actor, definitionID, reportrepo.ReportActionQuery)
	if err != nil {
		return nil, classifyReportRunStoreError(err)
	}
	queryName, err := reportInputQueryName([]byte(published.Version.InputSchemaJSON), conditionCode)
	if err != nil {
		return nil, fmt.Errorf("%w: report input query binding is invalid", ErrReportRunInvalid)
	}
	query, exists := service.config.Queries[queryName]
	if !exists {
		return nil, fmt.Errorf("%w: configured query is missing", ErrReportInputQueryUnavailable)
	}
	queryCtx, cancel := context.WithTimeout(ctx, service.config.Oracle.QueryTimeout)
	defer cancel()
	connection, err := service.connection(queryCtx)
	if err != nil {
		return nil, err
	}
	options, err := connection.QueryInputOptions(queryCtx, query.Select, exactName, reportInputOptionLimit)
	if err != nil {
		return nil, fmt.Errorf("%w: query Oracle options: %v", ErrReportInputQueryUnavailable, err)
	}
	return &ReportInputOptionListDTO{Items: options}, nil
}

func (service *ReportInputQueryService) connection(ctx context.Context) (reportInputOracleConnection, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	if service.oracle != nil {
		return service.oracle, nil
	}
	configured := service.config.Oracle
	connection, err := service.open(ctx, reportoracle.Config{
		Host: configured.Host, Port: configured.Port, ServiceName: configured.ServiceName, SID: configured.SID,
		Username: configured.Username, Password: configured.Password, Timezone: configured.Timezone,
		ConnectTimeout: configured.ConnectTimeout, MaxOpenConnections: configured.MaxOpenConnections,
		MaxIdleConnections: configured.MaxIdleConnections, ConnectionLifetime: configured.ConnectionLifetime,
		ConnectionIdleTime: configured.ConnectionIdleTime, PrefetchRows: configured.PrefetchRows, FetchArraySize: configured.ArraySize,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: open default Oracle: %v", ErrReportInputQueryUnavailable, err)
	}
	service.oracle = connection
	return connection, nil
}

func reportInputQueryName(schemaJSON []byte, conditionCode string) (string, error) {
	var schema map[string]json.RawMessage
	decoder := json.NewDecoder(bytes.NewReader(schemaJSON))
	if err := decoder.Decode(&schema); err != nil {
		return "", err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return "", errors.New("input schema contains trailing JSON")
	}
	encoded, exists := schema[conditionCode]
	if !exists {
		return "", errors.New("input condition does not exist")
	}
	var field struct {
		Control   string `json:"control"`
		QueryName string `json:"queryName"`
	}
	fieldDecoder := json.NewDecoder(bytes.NewReader(encoded))
	if err := fieldDecoder.Decode(&field); err != nil {
		return "", err
	}
	field.Control = strings.ToUpper(strings.TrimSpace(field.Control))
	field.QueryName = strings.TrimSpace(field.QueryName)
	if field.Control != "SELECT" || !reportInputQueryNamePattern.MatchString(field.QueryName) {
		return "", errors.New("input condition is not query-backed")
	}
	return field.QueryName, nil
}
