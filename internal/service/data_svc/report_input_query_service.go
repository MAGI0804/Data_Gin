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
	"time"
	"unicode/utf8"

	appConfig "gin-biz-web-api/config"
	"gin-biz-web-api/internal/reportoracle"
	"gin-biz-web-api/internal/reportrepo"
	"gin-biz-web-api/model"
)

var ErrReportInputQueryUnavailable = errors.New("report input query service: unavailable")

const reportInputOptionLimit = 500

type reportInputQueryStore interface {
	FindPublishedReport(context.Context, uint, uint, string) (*reportrepo.PublishedReport, error)
}

type reportInputQueryDefinitionStore interface {
	ListReportInputQueryDefinitions(context.Context) ([]model.ReportInputQueryDefinition, error)
	GetReportInputQueryDefinition(context.Context, uint) (*model.ReportInputQueryDefinition, error)
	FindReportInputQueryByName(context.Context, string) (*model.ReportInputQueryDefinition, error)
	CreateReportInputQueryDefinition(context.Context, uint, *model.ReportInputQueryDefinition) error
	UpdateReportInputQueryDefinition(context.Context, uint, *model.ReportInputQueryDefinition, uint64) error
	DeleteReportInputQueryDefinition(context.Context, uint, uint, uint64) error
	RecordReportInputQueryTest(context.Context, uint, uint, string, string, time.Time) error
}

type reportInputOracleConnection interface {
	QueryInputOptions(context.Context, string, string, int) ([]reportoracle.InputOption, error)
	Close() error
}

type reportInputOracleOpener func(context.Context, reportoracle.Config) (reportInputOracleConnection, error)

type ReportInputQueryService struct {
	store       reportInputQueryStore
	definitions reportInputQueryDefinitionStore
	config      appConfig.ReportInputQueryConfig
	open        reportInputOracleOpener
	now         func() time.Time
	mu          sync.Mutex
	oracle      reportInputOracleConnection
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
	repository := reportrepo.New()
	return NewReportInputQueryServiceWithStores(repository, repository, configured, func(ctx context.Context, config reportoracle.Config) (reportInputOracleConnection, error) {
		return reportoracle.Open(ctx, config)
	}), nil
}

func NewReportInputQueryServiceWithDependencies(store reportInputQueryStore, configured appConfig.ReportInputQueryConfig, opener reportInputOracleOpener) *ReportInputQueryService {
	return NewReportInputQueryServiceWithStores(store, nil, configured, opener)
}

func NewReportInputQueryServiceWithStores(store reportInputQueryStore, definitions reportInputQueryDefinitionStore, configured appConfig.ReportInputQueryConfig, opener reportInputOracleOpener) *ReportInputQueryService {
	if store == nil || opener == nil {
		panic("report input query service: dependencies are required")
	}
	if configured.Queries == nil {
		configured.Queries = map[string]appConfig.ReportInputQuery{}
	}
	return &ReportInputQueryService{store: store, definitions: definitions, config: configured, open: opener, now: func() time.Time { return time.Now().UTC() }}
}

func (service *ReportInputQueryService) List(ctx context.Context, actor uint) (*ReportInputQueryListDTO, error) {
	if service == nil || ctx == nil || actor == 0 {
		return nil, fmt.Errorf("%w: invalid list request", ErrReportInputQueryUnavailable)
	}
	unique := make(map[string]struct{}, len(service.config.Queries))
	for name := range service.config.Queries {
		unique[name] = struct{}{}
	}
	if service.definitions != nil {
		definitions, err := service.definitions.ListReportInputQueryDefinitions(ctx)
		if err != nil {
			return nil, fmt.Errorf("%w: list definitions: %v", ErrReportInputQueryUnavailable, err)
		}
		for _, definition := range definitions {
			if definition.Enabled {
				unique[definition.Name] = struct{}{}
			} else {
				delete(unique, definition.Name)
			}
		}
	}
	names := make([]string, 0, len(unique))
	for name := range unique {
		names = append(names, name)
	}
	sort.Strings(names)
	return &ReportInputQueryListDTO{Items: names}, nil
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
	statement := ""
	if service.definitions != nil {
		definition, definitionErr := service.definitions.FindReportInputQueryByName(ctx, queryName)
		switch {
		case definitionErr == nil && definition.Enabled:
			statement = definition.SelectSQL
		case definitionErr == nil:
			return nil, fmt.Errorf("%w: configured query is disabled", ErrReportInputQueryUnavailable)
		case !errors.Is(definitionErr, reportrepo.ErrInputQueryNotFound):
			return nil, fmt.Errorf("%w: load configured query: %v", ErrReportInputQueryUnavailable, definitionErr)
		}
	}
	if statement == "" {
		if query, exists := service.config.Queries[queryName]; exists {
			statement = query.Select
		}
	}
	if statement == "" {
		return nil, fmt.Errorf("%w: configured query is missing", ErrReportInputQueryUnavailable)
	}
	queryTimeout := service.config.Oracle.QueryTimeout
	if queryTimeout <= 0 {
		queryTimeout = 30 * time.Second
	}
	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	connection, err := service.connection(queryCtx)
	if err != nil {
		return nil, err
	}
	// Runtime selectors load the configured query once and perform fuzzy search
	// in the browser. Keep accepting exactName for backwards-compatible request
	// validation, but never turn it into an Oracle WHERE predicate here.
	options, err := connection.QueryInputOptions(queryCtx, statement, "", reportInputOptionLimit)
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
	if field.Control != "SELECT" || !appConfig.ValidateReportInputQueryName(field.QueryName) {
		return "", errors.New("input condition is not query-backed")
	}
	return field.QueryName, nil
}
