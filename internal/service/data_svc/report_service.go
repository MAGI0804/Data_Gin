package data_svc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"

	"gin-biz-web-api/internal/reporting"
	"gin-biz-web-api/internal/reportoracle"
	"gin-biz-web-api/internal/reportrepo"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/model"
)

const (
	defaultReportDraftPageSize = 50
	maxReportDraftPageSize     = 100
	maxReportParameters        = 128
	maxReportColumns           = 512
	maxReportGrants            = 256
)

var (
	ErrReportInvalid  = errors.New("report draft service: invalid input")
	ErrReportNotFound = errors.New("report draft service: not found")
	ErrReportConflict = errors.New("report draft service: conflict")

	reportCodePattern        = regexp.MustCompile(`^[a-z][a-z0-9_-]{2,63}$`)
	reportLogicalCodePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,63}$`)
	reportControlTypes       = map[string]struct{}{"TEXT": {}, "TEXTAREA": {}, "NUMBER": {}, "CHECKBOX": {}, "DATE": {}, "DATETIME": {}, "SELECT": {}, "MULTI_SELECT": {}}
	reportValueTypes         = map[string]struct{}{"string": {}, "integer": {}, "decimal": {}, "boolean": {}, "date": {}, "datetime": {}, "enum": {}, "multi_enum": {}, "json": {}}
	reportGrantActions       = map[string]struct{}{"QUERY": {}, "EXPORT": {}}
)

type reportDraftStore interface {
	CreateDraft(context.Context, uint, *reportrepo.Draft) error
	FindDraftByID(context.Context, uint, uint) (*reportrepo.Draft, error)
	ListDrafts(context.Context, uint, reportrepo.DraftListQuery) (reportrepo.DraftPage, error)
	UpdateDraft(context.Context, uint, uint, uint64, *reportrepo.Draft) error
}

type ReportDraftService struct {
	store reportDraftStore
}

func NewReportDraftService() *ReportDraftService {
	return NewReportDraftServiceWithStore(reportrepo.New())
}

func NewReportDraftServiceWithStore(store reportDraftStore) *ReportDraftService {
	if store == nil {
		panic("report draft service: nil store")
	}
	return &ReportDraftService{store: store}
}

type ReportDraftDTO struct {
	ID           uint                 `json:"id"`
	Code         string               `json:"code"`
	Name         string               `json:"name"`
	Category     string               `json:"category"`
	Description  string               `json:"description"`
	DatasourceID uint                 `json:"datasourceId"`
	Status       string               `json:"status"`
	LockVersion  uint64               `json:"lockVersion"`
	Procedure    ReportProcedureDTO   `json:"procedure"`
	Result       ReportResultDTO      `json:"result"`
	CallTemplate string               `json:"callTemplate"`
	Parameters   []ReportParameterDTO `json:"parameters"`
	Columns      []ReportColumnDTO    `json:"columns"`
	Grants       []ReportGrantDTO     `json:"grants"`
	CreatedAt    time.Time            `json:"createdAt"`
	UpdatedAt    time.Time            `json:"updatedAt"`
}

type ReportDraftSummaryDTO struct {
	ID           uint      `json:"id"`
	Code         string    `json:"code"`
	Name         string    `json:"name"`
	Category     string    `json:"category"`
	Description  string    `json:"description"`
	DatasourceID uint      `json:"datasourceId"`
	Status       string    `json:"status"`
	LockVersion  uint64    `json:"lockVersion"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type ReportProcedureDTO struct {
	Owner    string `json:"owner"`
	Package  string `json:"package,omitempty"`
	Name     string `json:"name"`
	Overload string `json:"overload,omitempty"`
}

type ReportResultDTO struct {
	TableOwner  string `json:"tableOwner"`
	TableName   string `json:"tableName"`
	RunIDColumn string `json:"runIdColumn"`
	RowIDColumn string `json:"rowIdColumn"`
}

type ReportParameterDTO struct {
	Code               string          `json:"code"`
	Label              string          `json:"label"`
	DisplayOrder       int             `json:"displayOrder"`
	ControlType        string          `json:"controlType"`
	LogicalType        string          `json:"logicalType"`
	Cardinality        string          `json:"cardinality"`
	ProcedureArgName   string          `json:"procedureArgName"`
	Position           int             `json:"position"`
	OracleType         string          `json:"oracleType"`
	Precision          *int            `json:"precision,omitempty"`
	Scale              *int            `json:"scale,omitempty"`
	MaxLength          *int            `json:"maxLength,omitempty"`
	Required           bool            `json:"required"`
	Nullable           bool            `json:"nullable"`
	SystemInjected     bool            `json:"systemInjected"`
	Sensitive          bool            `json:"sensitive"`
	DefaultValue       json.RawMessage `json:"defaultValue,omitempty"`
	AllowedValues      json.RawMessage `json:"allowedValues,omitempty"`
	Validation         json.RawMessage `json:"validation,omitempty"`
	Normalizer         json.RawMessage `json:"normalizer,omitempty"`
	ValueSource        json.RawMessage `json:"valueSource,omitempty"`
	Timezone           string          `json:"timezone,omitempty"`
	NullPolicy         string          `json:"nullPolicy"`
	CollectionEncoding string          `json:"collectionEncoding,omitempty"`
	ErrorMessage       string          `json:"errorMessage,omitempty"`
}

type ReportColumnDTO struct {
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
	NullDisplay       string          `json:"nullDisplay,omitempty"`
}

type ReportGrantDTO struct {
	SubjectType string          `json:"subjectType"`
	SubjectID   uint            `json:"subjectId"`
	Actions     json.RawMessage `json:"actions"`
}

type ReportDraftListDTO struct {
	Items       []ReportDraftSummaryDTO `json:"items"`
	HasMore     bool                    `json:"hasMore"`
	NextAfterID uint                    `json:"nextAfterId,omitempty"`
}

func (service *ReportDraftService) Create(ctx context.Context, actor uint, request requestbody.ReportDraftSaveRequest) (*ReportDraftDTO, error) {
	if service == nil || service.store == nil || ctx == nil || actor == 0 {
		return nil, invalidReport("service, context and actor are required")
	}
	if request.ExpectedLockVersion != nil {
		return nil, invalidReport("expectedLockVersion is only valid on update")
	}
	draft, err := reportDraftFromRequest(actor, request)
	if err != nil {
		return nil, err
	}
	if err := service.store.CreateDraft(ctx, actor, draft); err != nil {
		return nil, classifyReportStoreError(err)
	}
	persisted, err := service.store.FindDraftByID(ctx, actor, draft.Definition.ID)
	if err != nil {
		return nil, classifyReportStoreError(err)
	}
	return reportDraftDTO(persisted), nil
}

func (service *ReportDraftService) Get(ctx context.Context, actor, definitionID uint) (*ReportDraftDTO, error) {
	if service == nil || service.store == nil || ctx == nil || actor == 0 || definitionID == 0 {
		return nil, invalidReport("service, context, actor and report id are required")
	}
	draft, err := service.store.FindDraftByID(ctx, actor, definitionID)
	if err != nil {
		return nil, classifyReportStoreError(err)
	}
	return reportDraftDTO(draft), nil
}

func (service *ReportDraftService) List(ctx context.Context, actor, afterID uint, limit int, category, search string) (*ReportDraftListDTO, error) {
	if service == nil || service.store == nil || ctx == nil || actor == 0 {
		return nil, invalidReport("service, context and actor are required")
	}
	if limit == 0 {
		limit = defaultReportDraftPageSize
	}
	category = strings.TrimSpace(category)
	search = strings.TrimSpace(search)
	if limit < 1 || limit > maxReportDraftPageSize || utf8.RuneCountInString(category) > 64 || utf8.RuneCountInString(search) > 128 {
		return nil, invalidReport("invalid list filters")
	}
	page, err := service.store.ListDrafts(ctx, actor, reportrepo.DraftListQuery{
		AfterID: afterID, Limit: limit, Category: category, Search: search,
	})
	if err != nil {
		return nil, classifyReportStoreError(err)
	}
	result := &ReportDraftListDTO{Items: make([]ReportDraftSummaryDTO, 0, len(page.Items)), HasMore: page.HasMore, NextAfterID: page.NextAfterID}
	for _, item := range page.Items {
		result.Items = append(result.Items, reportDraftSummaryDTO(item))
	}
	return result, nil
}

func (service *ReportDraftService) Update(ctx context.Context, actor, definitionID uint, request requestbody.ReportDraftSaveRequest) (*ReportDraftDTO, error) {
	if service == nil || service.store == nil || ctx == nil || actor == 0 || definitionID == 0 {
		return nil, invalidReport("service, context, actor and report id are required")
	}
	if request.ExpectedLockVersion == nil || *request.ExpectedLockVersion == 0 {
		return nil, invalidReport("expectedLockVersion is required")
	}
	// Preserve 404 for a missing scoped report. A deletion or concurrent change
	// after this read is still correctly reported as a 409 by UpdateDraft.
	if _, err := service.store.FindDraftByID(ctx, actor, definitionID); err != nil {
		return nil, classifyReportStoreError(err)
	}
	draft, err := reportDraftFromRequest(actor, request)
	if err != nil {
		return nil, err
	}
	draft.Definition.ID = definitionID
	if err := service.store.UpdateDraft(ctx, actor, definitionID, *request.ExpectedLockVersion, draft); err != nil {
		return nil, classifyReportStoreError(err)
	}
	persisted, err := service.store.FindDraftByID(ctx, actor, definitionID)
	if err != nil {
		return nil, classifyReportStoreError(err)
	}
	return reportDraftDTO(persisted), nil
}

func reportDraftFromRequest(actor uint, request requestbody.ReportDraftSaveRequest) (*reportrepo.Draft, error) {
	request.Code = strings.ToLower(strings.TrimSpace(request.Code))
	request.Name = strings.TrimSpace(request.Name)
	request.Category = strings.TrimSpace(request.Category)
	request.Description = strings.TrimSpace(request.Description)
	if !reportCodePattern.MatchString(request.Code) || request.Name == "" || utf8.RuneCountInString(request.Name) > 128 ||
		utf8.RuneCountInString(request.Category) > 64 || utf8.RuneCountInString(request.Description) > 500 || request.DatasourceID == 0 {
		return nil, invalidReport("invalid report identity")
	}
	procedure, err := reportoracle.NormalizeProcedureRef(reportoracle.ProcedureRef{
		Owner: request.Procedure.Owner, Package: request.Procedure.Package,
		Name: request.Procedure.Name, Overload: request.Procedure.Overload,
	})
	if err != nil {
		return nil, invalidReport("invalid Oracle procedure")
	}
	resultRef, err := reportoracle.NormalizeResultTableRef(reportoracle.ResultTableRef{Owner: request.Result.TableOwner, Name: request.Result.TableName})
	if err != nil {
		return nil, invalidReport("invalid Oracle result table")
	}
	runIDColumn, err := normalizeReportIdentifier(request.Result.RunIDColumn)
	if err != nil {
		return nil, invalidReport("invalid result run id column")
	}
	rowIDColumn, err := normalizeReportIdentifier(request.Result.RowIDColumn)
	if err != nil || runIDColumn == rowIDColumn {
		return nil, invalidReport("invalid result row id column")
	}
	if len(procedure.Owner) > 128 || len(procedure.Package) > 128 || len(procedure.Name) > 128 || len(procedure.Overload) > 32 ||
		len(resultRef.Owner) > 128 || len(resultRef.Name) > 128 {
		return nil, invalidReport("Oracle object name is too long")
	}
	parameters, definitions, err := reportParametersFromRequest(request.Parameters)
	if err != nil {
		return nil, err
	}
	callTemplate := strings.TrimSpace(request.CallTemplate)
	if len(callTemplate) > 64*1024 {
		return nil, invalidReport("call template is too large")
	}
	if _, err := reporting.CompileCallTemplate(callTemplate, definitions); err != nil {
		return nil, invalidReport("call template and parameters do not match")
	}
	columns, err := reportColumnsFromRequest(request.Columns)
	if err != nil {
		return nil, err
	}
	grants, err := reportGrantsFromRequest(request.Grants, actor)
	if err != nil {
		return nil, err
	}
	return &reportrepo.Draft{
		Definition: model.ReportDefinition{
			Code: request.Code, Name: request.Name, Category: request.Category, Description: request.Description,
			DatasourceID: request.DatasourceID, OwnerUserID: actor, Status: model.ReportDefinitionStatusDraft,
			CreatedBy: actor, UpdatedBy: actor,
		},
		Version: model.ReportVersion{
			Status: model.ReportVersionStatusDraft, ProcedureOwner: procedure.Owner, PackageName: procedure.Package,
			ProcedureName: procedure.Name, ProcedureOverload: procedure.Overload,
			ResultTableOwner: resultRef.Owner, ResultTableName: resultRef.Name,
			ResultRunIDColumn: runIDColumn, ResultRowIDColumn: rowIDColumn,
			CallTemplate: callTemplate, CreatedBy: actor,
		},
		Parameters: parameters, Columns: columns, Grants: grants,
	}, nil
}

func reportParametersFromRequest(requests []requestbody.ReportParameterRequest) ([]model.ReportParameter, []reporting.ParameterDefinition, error) {
	if len(requests) == 0 || len(requests) > maxReportParameters {
		return nil, nil, invalidReport("parameters must contain between 1 and 128 items")
	}
	parameters := make([]model.ReportParameter, 0, len(requests))
	definitions := make([]reporting.ParameterDefinition, 0, len(requests))
	displayOrders := make(map[int]struct{}, len(requests))
	for _, request := range requests {
		request.Code = strings.TrimSpace(request.Code)
		request.Label = strings.TrimSpace(request.Label)
		request.ControlType = strings.ToUpper(strings.TrimSpace(request.ControlType))
		request.LogicalType = strings.ToLower(strings.TrimSpace(request.LogicalType))
		request.Cardinality = strings.ToUpper(strings.TrimSpace(request.Cardinality))
		request.ProcedureArgName = strings.TrimSpace(request.ProcedureArgName)
		request.OracleType = strings.ToUpper(strings.Join(strings.Fields(request.OracleType), " "))
		request.Timezone = strings.TrimSpace(request.Timezone)
		request.NullPolicy = strings.ToUpper(strings.TrimSpace(request.NullPolicy))
		request.CollectionEncoding = strings.ToUpper(strings.TrimSpace(request.CollectionEncoding))
		request.ErrorMessage = strings.TrimSpace(request.ErrorMessage)
		_, knownControlType := reportControlTypes[request.ControlType]
		if !reportLogicalCodePattern.MatchString(request.Code) || request.Label == "" || utf8.RuneCountInString(request.Label) > 128 ||
			!knownControlType || request.Position <= 0 || request.DisplayOrder < 0 ||
			request.OracleType == "" || utf8.RuneCountInString(request.OracleType) > 64 || utf8.RuneCountInString(request.ErrorMessage) > 500 {
			return nil, nil, invalidReport("invalid parameter configuration")
		}
		argName, err := normalizeReportIdentifier(request.ProcedureArgName)
		if err != nil {
			return nil, nil, invalidReport("invalid procedure argument name")
		}
		if request.Cardinality == "" {
			request.Cardinality = reporting.CardinalitySingle
		}
		if request.NullPolicy == "" {
			request.NullPolicy = "TYPED_NULL"
		}
		if request.Required && request.Nullable {
			return nil, nil, invalidReport("required parameter cannot be nullable")
		}
		if _, exists := displayOrders[request.DisplayOrder]; exists {
			return nil, nil, invalidReport("duplicated parameter display order")
		}
		displayOrders[request.DisplayOrder] = struct{}{}
		if request.Cardinality != reporting.CardinalitySingle && request.Cardinality != reporting.CardinalityMultiple {
			return nil, nil, invalidReport("invalid parameter cardinality")
		}
		if request.NullPolicy != "TYPED_NULL" {
			return nil, nil, invalidReport("invalid parameter null policy")
		}
		if request.Precision != nil && (*request.Precision < 1 || *request.Precision > 38) ||
			request.Scale != nil && (*request.Scale < -84 || *request.Scale > 127) ||
			request.MaxLength != nil && (*request.MaxLength < 1 || *request.MaxLength > 1_000_000) {
			return nil, nil, invalidReport("invalid parameter size constraint")
		}
		if err := validateOptionalJSON(request.DefaultValue, jsonAny); err != nil ||
			validateOptionalJSON(request.AllowedValues, jsonArray) != nil || validateOptionalJSON(request.Validation, jsonObject) != nil ||
			validateOptionalJSON(request.Normalizer, jsonObject) != nil || validateOptionalJSON(request.ValueSource, jsonObject) != nil {
			return nil, nil, invalidReport("invalid parameter JSON configuration")
		}
		definition := reporting.ParameterDefinition{
			Code: request.Code, ProcedureArgName: argName, Position: request.Position, Direction: "IN",
			LogicalType: request.LogicalType, OracleType: request.OracleType, Cardinality: request.Cardinality,
			Required: request.Required, Nullable: request.Nullable, SystemInjected: request.SystemInjected,
			Sensitive: request.Sensitive, DefaultValue: cloneJSON(request.DefaultValue), AllowedValues: cloneJSON(request.AllowedValues),
			Validation: cloneJSON(request.Validation), Timezone: request.Timezone, NullPolicy: request.NullPolicy,
			CollectionEncoding: request.CollectionEncoding,
		}
		parameters = append(parameters, model.ReportParameter{
			ParameterCode: request.Code, Label: request.Label, DisplayOrder: request.DisplayOrder,
			ControlType: request.ControlType, LogicalType: request.LogicalType, Cardinality: request.Cardinality,
			ProcedureArgName: argName, Position: request.Position, Direction: "IN", OracleType: request.OracleType,
			PrecisionValue: request.Precision, ScaleValue: request.Scale, MaxLength: request.MaxLength,
			Required: request.Required, Nullable: request.Nullable, SystemInjected: request.SystemInjected, Sensitive: request.Sensitive,
			DefaultValueJSON: jsonText(request.DefaultValue), AllowedValuesJSON: jsonText(request.AllowedValues),
			ValidationJSON: jsonText(request.Validation), NormalizerJSON: jsonText(request.Normalizer),
			ValueSourceJSON: jsonText(request.ValueSource), Timezone: request.Timezone, NullPolicy: request.NullPolicy,
			CollectionEncoding: request.CollectionEncoding, ErrorMessage: request.ErrorMessage,
		})
		definitions = append(definitions, definition)
	}
	if _, err := reportoracle.BuildCallPlan(reportoracle.ProcedureRef{Owner: "VALIDATION", Name: "VALIDATION"}, definitions); err != nil {
		return nil, nil, invalidReport("parameter binding configuration is invalid")
	}
	if err := reporting.ValidateParameterDefinitions(definitions); err != nil {
		return nil, nil, invalidReport("parameter validation configuration is invalid")
	}
	return parameters, definitions, nil
}

func reportColumnsFromRequest(requests []requestbody.ReportColumnRequest) ([]model.ReportColumn, error) {
	if len(requests) == 0 || len(requests) > maxReportColumns {
		return nil, invalidReport("columns must contain between 1 and 512 items")
	}
	columns := make([]model.ReportColumn, 0, len(requests))
	logicalCodes := make(map[string]struct{}, len(requests))
	fieldIDs := make(map[string]struct{}, len(requests))
	databaseColumns := make(map[string]struct{}, len(requests))
	excelHeaders := make(map[string]struct{}, len(requests))
	displayOrders := make(map[int]struct{}, len(requests))
	exportOrders := make(map[int]struct{}, len(requests))
	for _, request := range requests {
		request.FieldID = strings.TrimSpace(request.FieldID)
		request.LogicalCode = strings.TrimSpace(request.LogicalCode)
		request.DatabaseColumn = strings.TrimSpace(request.DatabaseColumn)
		request.SourceOracleType = strings.ToUpper(strings.Join(strings.Fields(request.SourceOracleType), " "))
		request.ValueType = strings.ToLower(strings.TrimSpace(request.ValueType))
		request.PreviewHeader = strings.TrimSpace(request.PreviewHeader)
		request.ExcelHeader = strings.TrimSpace(request.ExcelHeader)
		request.NullDisplay = strings.TrimSpace(request.NullDisplay)
		fieldID, fieldIDError := uuid.Parse(request.FieldID)
		_, knownValueType := reportValueTypes[request.ValueType]
		if fieldIDError != nil || len(request.FieldID) != 36 || !reportLogicalCodePattern.MatchString(request.LogicalCode) ||
			request.SourceOracleType == "" || len(request.SourceOracleType) > 64 || !knownValueType || request.DisplayOrder < 0 || request.ExportOrder < 0 ||
			request.ExcelWidth < 0 || request.ExcelWidth > 255 || utf8.RuneCountInString(request.PreviewHeader) > 255 ||
			utf8.RuneCountInString(request.ExcelHeader) > 255 || utf8.RuneCountInString(request.NullDisplay) > 64 {
			return nil, invalidReport("invalid result column configuration")
		}
		if request.Precision != nil && (*request.Precision < 1 || *request.Precision > 38) ||
			request.Scale != nil && (*request.Scale < -84 || *request.Scale > 127) {
			return nil, invalidReport("invalid result column size constraint")
		}
		request.FieldID = fieldID.String()
		databaseColumn, err := normalizeReportIdentifier(request.DatabaseColumn)
		if err != nil {
			return nil, invalidReport("invalid Oracle result column")
		}
		logicalKey := strings.ToUpper(request.LogicalCode)
		if _, exists := logicalCodes[logicalKey]; exists {
			return nil, invalidReport("duplicated logical column")
		}
		if _, exists := fieldIDs[request.FieldID]; exists {
			return nil, invalidReport("duplicated stable field id")
		}
		if _, exists := databaseColumns[databaseColumn]; exists {
			return nil, invalidReport("duplicated Oracle result column")
		}
		if _, exists := displayOrders[request.DisplayOrder]; exists {
			return nil, invalidReport("duplicated column display order")
		}
		if request.ExportVisible {
			if _, exists := exportOrders[request.ExportOrder]; exists {
				return nil, invalidReport("duplicated column export order")
			}
			exportOrders[request.ExportOrder] = struct{}{}
		}
		if request.PreviewVisible && request.PreviewHeader == "" {
			return nil, invalidReport("visible preview column requires a header")
		}
		if request.ExportVisible {
			if request.ExcelHeader == "" {
				return nil, invalidReport("visible export column requires an Excel header")
			}
			if _, exists := excelHeaders[request.ExcelHeader]; exists {
				return nil, invalidReport("duplicated Excel header")
			}
			excelHeaders[request.ExcelHeader] = struct{}{}
		}
		if validateOptionalJSON(request.AllowedOperators, jsonArray) != nil || validateOptionalJSON(request.Format, jsonObject) != nil ||
			validateOptionalJSON(request.DictionaryVersion, jsonObject) != nil || validateOptionalJSON(request.MaskingPolicy, jsonObject) != nil {
			return nil, invalidReport("invalid result column JSON configuration")
		}
		logicalCodes[logicalKey] = struct{}{}
		fieldIDs[request.FieldID] = struct{}{}
		databaseColumns[databaseColumn] = struct{}{}
		displayOrders[request.DisplayOrder] = struct{}{}
		columns = append(columns, model.ReportColumn{
			FieldID: request.FieldID, LogicalCode: request.LogicalCode, DatabaseColumn: databaseColumn,
			SourceOracleType: request.SourceOracleType, PrecisionValue: request.Precision, ScaleValue: request.Scale,
			Nullable: request.Nullable, ValueType: request.ValueType, PreviewHeader: request.PreviewHeader,
			ExcelHeader: request.ExcelHeader, DisplayOrder: request.DisplayOrder, ExportOrder: request.ExportOrder,
			PreviewVisible: request.PreviewVisible, ExportVisible: request.ExportVisible, Filterable: request.Filterable,
			Sortable: request.Sortable, ExportAllowed: request.ExportAllowed,
			AllowedOperatorsJSON: jsonText(request.AllowedOperators), FormatJSON: jsonText(request.Format),
			DictionaryVersionJSON: jsonText(request.DictionaryVersion), MaskingPolicyJSON: jsonText(request.MaskingPolicy),
			ExcelWidth: request.ExcelWidth, NullDisplay: request.NullDisplay,
		})
	}
	sort.SliceStable(columns, func(i, j int) bool { return columns[i].DisplayOrder < columns[j].DisplayOrder })
	return columns, nil
}

func reportGrantsFromRequest(requests []requestbody.ReportGrantRequest, actor uint) ([]model.ReportGrant, error) {
	if len(requests) > maxReportGrants {
		return nil, invalidReport("too many grants")
	}
	grants := make([]model.ReportGrant, 0, len(requests))
	seen := make(map[string]struct{}, len(requests))
	for _, request := range requests {
		request.SubjectType = strings.ToUpper(strings.TrimSpace(request.SubjectType))
		if (request.SubjectType != "USER" && request.SubjectType != "ROLE") || request.SubjectID == 0 {
			return nil, invalidReport("invalid grant subject")
		}
		actions, err := normalizeReportActions(request.Actions)
		if err != nil {
			return nil, err
		}
		key := fmt.Sprintf("%s:%d", request.SubjectType, request.SubjectID)
		if _, exists := seen[key]; exists {
			return nil, invalidReport("duplicated grant subject")
		}
		seen[key] = struct{}{}
		grants = append(grants, model.ReportGrant{SubjectType: request.SubjectType, SubjectID: request.SubjectID, ActionsJSON: model.JSONText(actions), CreatedBy: actor, UpdatedBy: actor})
	}
	return grants, nil
}

func normalizeReportActions(raw json.RawMessage) ([]byte, error) {
	var actions []string
	if err := decodeStrictJSON(raw, &actions); err != nil || len(actions) == 0 || len(actions) > 8 {
		return nil, invalidReport("grant actions must be a non-empty string array")
	}
	seen := make(map[string]struct{}, len(actions))
	for index, action := range actions {
		action = strings.ToUpper(strings.TrimSpace(action))
		if _, allowed := reportGrantActions[action]; !allowed {
			return nil, invalidReport("invalid grant action")
		}
		if _, exists := seen[action]; exists {
			return nil, invalidReport("duplicated grant action")
		}
		seen[action] = struct{}{}
		actions[index] = action
	}
	return json.Marshal(actions)
}

type jsonKind uint8

const (
	jsonAny jsonKind = iota
	jsonArray
	jsonObject
)

func validateOptionalJSON(raw json.RawMessage, kind jsonKind) error {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	var value interface{}
	if err := decodeStrictJSON(raw, &value); err != nil {
		return err
	}
	switch kind {
	case jsonArray:
		if _, ok := value.([]interface{}); !ok {
			return errors.New("not array")
		}
	case jsonObject:
		if _, ok := value.(map[string]interface{}); !ok {
			return errors.New("not object")
		}
	}
	return nil
}

func decodeStrictJSON(raw []byte, destination interface{}) error {
	if len(bytes.TrimSpace(raw)) == 0 {
		return io.ErrUnexpectedEOF
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("multiple JSON values")
	}
	return nil
}

func normalizeReportIdentifier(value string) (string, error) {
	ref, err := reportoracle.NormalizeResultTableRef(reportoracle.ResultTableRef{Owner: "VALIDATION", Name: value})
	if err != nil {
		return "", err
	}
	return ref.Name, nil
}

func classifyReportStoreError(err error) error {
	switch {
	case errors.Is(err, reportrepo.ErrDraftNotFound):
		return ErrReportNotFound
	case errors.Is(err, reportrepo.ErrDraftVersionConflict), isReportDuplicateError(err):
		return ErrReportConflict
	case errors.Is(err, reportrepo.ErrInvalidDraft):
		return invalidReport("draft repository rejected the contract")
	default:
		return fmt.Errorf("report draft service: store: %w", err)
	}
}

func isReportDuplicateError(err error) bool {
	var mysqlError *mysqlDriver.MySQLError
	return errors.As(err, &mysqlError) && mysqlError.Number == 1062
}

func invalidReport(message string) error { return fmt.Errorf("%w: %s", ErrReportInvalid, message) }

func jsonText(raw json.RawMessage) model.JSONText { return model.JSONText(cloneJSON(raw)) }

func cloneJSON(raw []byte) json.RawMessage {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	return append(json.RawMessage(nil), raw...)
}

func reportDraftDTO(draft *reportrepo.Draft) *ReportDraftDTO {
	if draft == nil {
		return nil
	}
	result := &ReportDraftDTO{
		ID: draft.Definition.ID, Code: draft.Definition.Code, Name: draft.Definition.Name,
		Category: draft.Definition.Category, Description: draft.Definition.Description,
		DatasourceID: draft.Definition.DatasourceID, Status: draft.Definition.Status, LockVersion: draft.LockVersion,
		Procedure:    ReportProcedureDTO{Owner: draft.Version.ProcedureOwner, Package: draft.Version.PackageName, Name: draft.Version.ProcedureName, Overload: draft.Version.ProcedureOverload},
		Result:       ReportResultDTO{TableOwner: draft.Version.ResultTableOwner, TableName: draft.Version.ResultTableName, RunIDColumn: draft.Version.ResultRunIDColumn, RowIDColumn: draft.Version.ResultRowIDColumn},
		CallTemplate: draft.Version.CallTemplate, Parameters: make([]ReportParameterDTO, 0, len(draft.Parameters)),
		Columns: make([]ReportColumnDTO, 0, len(draft.Columns)), Grants: make([]ReportGrantDTO, 0, len(draft.Grants)),
		CreatedAt: draft.Definition.CreatedAt, UpdatedAt: draft.Definition.UpdatedAt,
	}
	for _, parameter := range draft.Parameters {
		item := ReportParameterDTO{
			Code: parameter.ParameterCode, Label: parameter.Label, DisplayOrder: parameter.DisplayOrder,
			ControlType: parameter.ControlType, LogicalType: parameter.LogicalType, Cardinality: parameter.Cardinality,
			ProcedureArgName: parameter.ProcedureArgName, Position: parameter.Position, OracleType: parameter.OracleType,
			Precision: parameter.PrecisionValue, Scale: parameter.ScaleValue, MaxLength: parameter.MaxLength,
			Required: parameter.Required, Nullable: parameter.Nullable, SystemInjected: parameter.SystemInjected,
			Sensitive: parameter.Sensitive, AllowedValues: cloneJSON([]byte(parameter.AllowedValuesJSON)),
			Validation: cloneJSON([]byte(parameter.ValidationJSON)), Normalizer: cloneJSON([]byte(parameter.NormalizerJSON)),
			ValueSource: cloneJSON([]byte(parameter.ValueSourceJSON)), Timezone: parameter.Timezone,
			NullPolicy: parameter.NullPolicy, CollectionEncoding: parameter.CollectionEncoding, ErrorMessage: parameter.ErrorMessage,
		}
		if !parameter.Sensitive {
			item.DefaultValue = cloneJSON([]byte(parameter.DefaultValueJSON))
		}
		result.Parameters = append(result.Parameters, item)
	}
	for _, column := range draft.Columns {
		result.Columns = append(result.Columns, ReportColumnDTO{
			FieldID: column.FieldID, LogicalCode: column.LogicalCode, DatabaseColumn: column.DatabaseColumn,
			SourceOracleType: column.SourceOracleType, Precision: column.PrecisionValue, Scale: column.ScaleValue,
			Nullable: column.Nullable, ValueType: column.ValueType, PreviewHeader: column.PreviewHeader, ExcelHeader: column.ExcelHeader,
			DisplayOrder: column.DisplayOrder, ExportOrder: column.ExportOrder, PreviewVisible: column.PreviewVisible,
			ExportVisible: column.ExportVisible, Filterable: column.Filterable, Sortable: column.Sortable,
			ExportAllowed: column.ExportAllowed, AllowedOperators: cloneJSON([]byte(column.AllowedOperatorsJSON)),
			Format: cloneJSON([]byte(column.FormatJSON)), DictionaryVersion: cloneJSON([]byte(column.DictionaryVersionJSON)),
			MaskingPolicy: cloneJSON([]byte(column.MaskingPolicyJSON)), ExcelWidth: column.ExcelWidth, NullDisplay: column.NullDisplay,
		})
	}
	for _, grant := range draft.Grants {
		result.Grants = append(result.Grants, ReportGrantDTO{SubjectType: grant.SubjectType, SubjectID: grant.SubjectID, Actions: cloneJSON([]byte(grant.ActionsJSON))})
	}
	return result
}

func reportDraftSummaryDTO(summary reportrepo.DraftSummary) ReportDraftSummaryDTO {
	return ReportDraftSummaryDTO{
		ID: summary.Definition.ID, Code: summary.Definition.Code, Name: summary.Definition.Name,
		Category: summary.Definition.Category, Description: summary.Definition.Description,
		DatasourceID: summary.Definition.DatasourceID, Status: summary.Definition.Status,
		LockVersion: summary.LockVersion, CreatedAt: summary.Definition.CreatedAt, UpdatedAt: summary.Definition.UpdatedAt,
	}
}
