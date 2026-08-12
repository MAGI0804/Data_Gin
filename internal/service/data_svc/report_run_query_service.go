package data_svc

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"gin-biz-web-api/internal/reportoracle"
	"gin-biz-web-api/internal/reportrepo"
	"gin-biz-web-api/internal/reportsecret"
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/config"
)

var (
	ErrReportRunQueryInvalid    = errors.New("report run query service: invalid input")
	ErrReportRunQueryNotFound   = errors.New("report run query service: not found")
	ErrReportRunQueryConflict   = errors.New("report run query service: conflict")
	ErrReportRunResultTemporary = errors.New("report run query service: result temporarily unavailable")
)

const maxReportResultCursorLength = 1024

type reportRunQueryStore interface {
	FindRunForActor(context.Context, uint, uint) (*model.ReportRun, error)
	RequestRunCancellation(context.Context, uint, uint, time.Time) (*model.ReportRun, error)
	LoadResultContractForActor(context.Context, uint, uint, time.Time) (*reportrepo.RunResultContract, error)
}

type reportResultPageReader interface {
	Read(context.Context, reportrepo.RunResultContract, string, []string, *int64, int) (reportoracle.ResultPage, error)
}

type ReportRunViewDTO struct {
	ID              uint       `json:"id"`
	RunUUID         string     `json:"runUuid"`
	DefinitionID    uint       `json:"definitionId"`
	VersionID       uint       `json:"versionId"`
	Status          string     `json:"status"`
	RowCount        int64      `json:"rowCount"`
	CancelRequested bool       `json:"cancelRequested"`
	Attempt         int        `json:"attempt"`
	CreatedAt       time.Time  `json:"createdAt"`
	StartedAt       *time.Time `json:"startedAt,omitempty"`
	FinishedAt      *time.Time `json:"finishedAt,omitempty"`
	ResultExpiresAt *time.Time `json:"resultExpiresAt,omitempty"`
	ErrorCode       string     `json:"errorCode,omitempty"`
	ErrorMessage    string     `json:"errorMessage,omitempty"`
	CanCancel       bool       `json:"canCancel"`
	ResultAvailable bool       `json:"resultAvailable"`
}

type ReportResultColumnDTO struct {
	FieldID     string `json:"fieldId"`
	Code        string `json:"code"`
	Header      string `json:"header"`
	ValueType   string `json:"valueType"`
	Nullable    bool   `json:"nullable"`
	NullDisplay string `json:"nullDisplay,omitempty"`
}

type ReportResultRowDTO struct {
	Key    string                 `json:"key"`
	Values map[string]interface{} `json:"values"`
}

type ReportResultPaginationDTO struct {
	PageSize   int    `json:"pageSize"`
	HasMore    bool   `json:"hasMore"`
	NextCursor string `json:"nextCursor,omitempty"`
}

type ReportResultPageDTO struct {
	Run        ReportRunViewDTO          `json:"run"`
	Columns    []ReportResultColumnDTO   `json:"columns"`
	Rows       []ReportResultRowDTO      `json:"rows"`
	Pagination ReportResultPaginationDTO `json:"pagination"`
}

type ReportRunQueryService struct {
	store      reportRunQueryStore
	credential reportDatasourceCredentialDecryptor
	reader     reportResultPageReader
	cursorKey  []byte
	now        func() time.Time
}

func NewReportRunQueryService() *ReportRunQueryService {
	jwtKey := config.GetString("cfg.jwt.key")
	if strings.TrimSpace(jwtKey) == "" {
		panic("report run query service: JWT signing key is required")
	}
	key := sha256.Sum256([]byte("report-result-cursor\x1f" + jwtKey))
	return NewReportRunQueryServiceWithDependencies(reportrepo.New(), reportsecret.EnvironmentKeyring{}, oracleReportResultPageReader{}, key[:])
}

func NewReportRunQueryServiceWithDependencies(store reportRunQueryStore, credential reportDatasourceCredentialDecryptor, reader reportResultPageReader, cursorKey []byte) *ReportRunQueryService {
	if store == nil || credential == nil || reader == nil || len(cursorKey) < 8 {
		panic("report run query service: dependencies and cursor key are required")
	}
	return &ReportRunQueryService{store: store, credential: credential, reader: reader, cursorKey: append([]byte(nil), cursorKey...), now: func() time.Time { return time.Now().UTC() }}
}

func (service *ReportRunQueryService) Get(ctx context.Context, actor, runID uint) (*ReportRunViewDTO, error) {
	if service == nil || ctx == nil || actor == 0 || runID == 0 {
		return nil, ErrReportRunQueryInvalid
	}
	run, err := service.store.FindRunForActor(ctx, actor, runID)
	if err != nil {
		return nil, classifyReportRunQueryError(err)
	}
	result := reportRunView(*run, service.now())
	return &result, nil
}

func (service *ReportRunQueryService) Cancel(ctx context.Context, actor, runID uint) (*ReportRunViewDTO, error) {
	if service == nil || ctx == nil || actor == 0 || runID == 0 {
		return nil, ErrReportRunQueryInvalid
	}
	run, err := service.store.RequestRunCancellation(ctx, actor, runID, service.now())
	if err != nil {
		return nil, classifyReportRunQueryError(err)
	}
	result := reportRunView(*run, service.now())
	return &result, nil
}

func (service *ReportRunQueryService) ReadResults(ctx context.Context, actor, runID uint, cursor string, limit int) (*ReportResultPageDTO, error) {
	if service == nil || ctx == nil || actor == 0 || runID == 0 || limit < 1 || limit > 1000 || len(cursor) > maxReportResultCursorLength {
		return nil, ErrReportRunQueryInvalid
	}
	contract, err := service.store.LoadResultContractForActor(ctx, actor, runID, service.now())
	if err != nil {
		return nil, classifyReportRunQueryError(err)
	}
	columns, err := frozenPreviewColumns(contract.Run.PresentationSnapshotJSON)
	if err != nil || len(columns) == 0 {
		return nil, ErrReportRunQueryConflict
	}
	var after *int64
	if strings.TrimSpace(cursor) != "" {
		decoded, decodeErr := service.decodeCursor(cursor, contract.Run, limit)
		if decodeErr != nil {
			return nil, ErrReportRunQueryInvalid
		}
		after = &decoded.AfterRowID
	}
	password, err := service.credential.Decrypt(contract.Datasource.CredentialKeyVersion, contract.Datasource.PasswordCiphertext)
	if err != nil {
		return nil, ErrReportRunResultTemporary
	}
	databaseColumns := make([]string, len(columns))
	for index := range columns {
		databaseColumns[index] = columns[index].DatabaseColumn
	}
	page, err := service.reader.Read(ctx, *contract, password, databaseColumns, after, limit)
	if err != nil {
		return nil, ErrReportRunResultTemporary
	}
	result := &ReportResultPageDTO{
		Run: reportRunView(contract.Run, service.now()), Columns: make([]ReportResultColumnDTO, len(columns)),
		Rows: make([]ReportResultRowDTO, 0, len(page.Rows)), Pagination: ReportResultPaginationDTO{PageSize: limit, HasMore: page.HasNext},
	}
	for index, column := range columns {
		result.Columns[index] = ReportResultColumnDTO{FieldID: column.FieldID, Code: column.LogicalCode, Header: column.PreviewHeader, ValueType: column.ValueType, Nullable: column.Nullable, NullDisplay: column.NullDisplay}
	}
	for _, row := range page.Rows {
		if len(row.Values) != len(columns) {
			return nil, ErrReportRunResultTemporary
		}
		values := make(map[string]interface{}, len(columns))
		for index, column := range columns {
			values[column.LogicalCode] = reportResultValue(row.Values[index], column)
		}
		result.Rows = append(result.Rows, ReportResultRowDTO{Key: strconv.FormatInt(row.RowID, 10), Values: values})
	}
	if page.HasNext {
		result.Pagination.NextCursor, err = service.encodeCursor(reportResultCursor{Version: 1, RunUUID: contract.Run.RunUUID, ContractHash: contract.Run.ContractHash, PageSize: limit, AfterRowID: page.NextRowID})
		if err != nil {
			return nil, ErrReportRunResultTemporary
		}
	}
	return result, nil
}

type frozenResultColumn struct {
	FieldID           string          `json:"fieldId"`
	LogicalCode       string          `json:"logicalCode"`
	DatabaseColumn    string          `json:"databaseColumn"`
	SourceOracleType  string          `json:"sourceOracleType"`
	Precision         *int            `json:"precision,omitempty"`
	Scale             *int            `json:"scale,omitempty"`
	Nullable          bool            `json:"nullable"`
	ValueType         string          `json:"valueType"`
	PreviewHeader     string          `json:"previewHeader"`
	ExcelHeader       string          `json:"excelHeader"`
	DisplayOrder      int             `json:"displayOrder"`
	ExportOrder       int             `json:"exportOrder"`
	PreviewVisible    bool            `json:"previewVisible"`
	ExportVisible     bool            `json:"exportVisible"`
	Filterable        bool            `json:"filterable"`
	Sortable          bool            `json:"sortable"`
	ExportAllowed     bool            `json:"exportAllowed"`
	AllowedOperators  json.RawMessage `json:"allowedOperators,omitempty"`
	Format            json.RawMessage `json:"format,omitempty"`
	DictionaryVersion json.RawMessage `json:"dictionaryVersion,omitempty"`
	MaskingPolicy     json.RawMessage `json:"maskingPolicy,omitempty"`
	ExcelWidth        float64         `json:"excelWidth"`
	NullDisplay       string          `json:"nullDisplay"`
}

func frozenPreviewColumns(raw model.JSONText) ([]frozenResultColumn, error) {
	var columns []frozenResultColumn
	decoder := json.NewDecoder(bytes.NewReader([]byte(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&columns); err != nil {
		return nil, err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("presentation snapshot contains trailing data")
	}
	visible := columns[:0]
	for _, column := range columns {
		if column.PreviewVisible && column.FieldID != "" && column.LogicalCode != "" && column.DatabaseColumn != "" {
			visible = append(visible, column)
		}
	}
	return visible, nil
}

func reportRunView(run model.ReportRun, now time.Time) ReportRunViewDTO {
	available := run.Status == model.ReportRunStatusSucceeded && run.ResultPurgedAt == nil && run.ResultExpiresAt != nil && now.Before(*run.ResultExpiresAt)
	return ReportRunViewDTO{
		ID: run.ID, RunUUID: run.RunUUID, DefinitionID: run.DefinitionID, VersionID: run.VersionID, Status: run.Status,
		RowCount: run.RowCount, CancelRequested: run.CancelRequested, Attempt: run.Attempt, CreatedAt: run.CreatedAt,
		StartedAt: run.StartedAt, FinishedAt: run.FinishedAt, ResultExpiresAt: run.ResultExpiresAt,
		ErrorCode: run.ErrorCode, ErrorMessage: run.ErrorMessageSafe,
		CanCancel: run.Status == model.ReportRunStatusQueued || (run.Status == model.ReportRunStatusRunning && !run.CancelRequested), ResultAvailable: available,
	}
}

func reportResultValue(value interface{}, column frozenResultColumn) interface{} {
	if value == nil {
		return nil
	}
	if policy := bytes.TrimSpace(column.MaskingPolicy); len(policy) > 0 && !bytes.Equal(policy, []byte("{}")) && !bytes.Equal(policy, []byte("null")) {
		return "***"
	}
	switch typed := value.(type) {
	case []byte:
		return string(typed)
	case time.Time:
		return typed.UTC().Format(time.RFC3339Nano)
	}
	switch strings.ToLower(column.ValueType) {
	case "integer", "decimal", "number":
		return fmt.Sprint(value)
	default:
		return value
	}
}

type reportResultCursor struct {
	Version      int    `json:"v"`
	RunUUID      string `json:"runUuid"`
	ContractHash string `json:"contractHash"`
	PageSize     int    `json:"pageSize"`
	AfterRowID   int64  `json:"afterRowId"`
}

func (service *ReportRunQueryService) encodeCursor(cursor reportResultCursor) (string, error) {
	payload, err := json.Marshal(cursor)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, service.cursorKey)
	_, _ = mac.Write(payload)
	return base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func (service *ReportRunQueryService) decodeCursor(value string, run model.ReportRun, limit int) (reportResultCursor, error) {
	parts := strings.Split(value, ".")
	if len(parts) != 2 {
		return reportResultCursor{}, ErrReportRunQueryInvalid
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return reportResultCursor{}, err
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return reportResultCursor{}, err
	}
	mac := hmac.New(sha256.New, service.cursorKey)
	_, _ = mac.Write(payload)
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return reportResultCursor{}, ErrReportRunQueryInvalid
	}
	var cursor reportResultCursor
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cursor); err != nil || cursor.Version != 1 || cursor.RunUUID != run.RunUUID || cursor.ContractHash != run.ContractHash || cursor.PageSize != limit {
		return reportResultCursor{}, ErrReportRunQueryInvalid
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return reportResultCursor{}, ErrReportRunQueryInvalid
	}
	return cursor, nil
}

func classifyReportRunQueryError(err error) error {
	switch {
	case errors.Is(err, reportrepo.ErrReportRunAccessNotFound):
		return ErrReportRunQueryNotFound
	case errors.Is(err, reportrepo.ErrReportRunStateConflict), errors.Is(err, reportrepo.ErrReportResultUnavailable):
		return ErrReportRunQueryConflict
	default:
		return fmt.Errorf("report run query service: store: %w", err)
	}
}

type oracleReportResultPageReader struct{}

func (oracleReportResultPageReader) Read(ctx context.Context, contract reportrepo.RunResultContract, password string, columns []string, after *int64, limit int) (reportoracle.ResultPage, error) {
	adapter, err := reportoracle.Open(ctx, oracleConfigFromDatasource(contract.Datasource, password))
	if err != nil {
		return reportoracle.ResultPage{}, err
	}
	defer func() { _ = adapter.Close() }()
	ref := reportoracle.ResultSnapshotRef{
		Table:       reportoracle.ResultTableRef{Owner: contract.Version.ResultTableOwner, Name: contract.Version.ResultTableName},
		RunIDColumn: contract.Version.ResultRunIDColumn, RowIDColumn: contract.Version.ResultRowIDColumn, Columns: columns,
	}
	snapshot, err := adapter.InspectResultSnapshotContract(ctx, ref)
	if err != nil {
		return reportoracle.ResultPage{}, err
	}
	plan, err := reportoracle.BuildResultPagePlan(snapshot, columns)
	if err != nil {
		return reportoracle.ResultPage{}, err
	}
	return adapter.ReadResultPage(ctx, plan, contract.Run.RunUUID, after, limit)
}
