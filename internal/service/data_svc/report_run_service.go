package data_svc

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"gin-biz-web-api/internal/reporting"
	"gin-biz-web-api/internal/reportrepo"
	"gin-biz-web-api/internal/reportsecret"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/model"
)

var (
	ErrReportRunInvalid               = errors.New("report run service: invalid input")
	ErrReportRunDenied                = errors.New("report run service: forbidden")
	ErrReportRunCredentialUnavailable = errors.New("report run service: credential configuration unavailable")
)

var refreshNoncePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

const maxReportJSONConditionsBytes = 1024 * 1024

type reportRunStore interface {
	FindPublishedReport(context.Context, uint, uint, string) (*reportrepo.PublishedReport, error)
	CreateRun(context.Context, uint, uint, *reportrepo.CreateRunCommand) error
}

type reportParameterCipher interface {
	Encrypt([]byte) (string, string, error)
}

type ReportRunService struct {
	store  reportRunStore
	cipher reportParameterCipher
	now    func() time.Time
}

type ReportRunDTO struct {
	ID           uint      `json:"id"`
	RunUUID      string    `json:"runUuid"`
	DefinitionID uint      `json:"definitionId"`
	VersionID    uint      `json:"versionId"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"createdAt"`
}

// ReportRunContractDTO is the immutable, published input contract used by the
// query screen. It deliberately omits procedure and result-table metadata so
// clients can render inputs without learning Oracle implementation details.
type ReportRunContractDTO struct {
	DefinitionID  uint                 `json:"definitionId"`
	VersionID     uint                 `json:"versionId"`
	Code          string               `json:"code"`
	Name          string               `json:"name"`
	Description   string               `json:"description"`
	ExecutionMode string               `json:"executionMode"`
	InputSchema   json.RawMessage      `json:"inputSchema,omitempty"`
	Parameters    []ReportParameterDTO `json:"parameters"`
}

func NewReportRunService() *ReportRunService {
	return NewReportRunServiceWithDependencies(reportrepo.New(), reportsecret.EnvironmentParameterCipher{})
}

func NewReportRunServiceWithDependencies(store reportRunStore, cipher reportParameterCipher) *ReportRunService {
	if store == nil || cipher == nil {
		panic("report run service: dependencies are required")
	}
	return &ReportRunService{store: store, cipher: cipher, now: func() time.Time { return time.Now().UTC() }}
}

func (service *ReportRunService) Contract(ctx context.Context, actor, definitionID uint) (*ReportRunContractDTO, error) {
	if service == nil || service.store == nil || ctx == nil || actor == 0 || definitionID == 0 {
		return nil, fmt.Errorf("%w: actor and report are required", ErrReportRunInvalid)
	}
	published, err := service.store.FindPublishedReport(ctx, actor, definitionID, reportrepo.ReportActionQuery)
	if err != nil {
		return nil, classifyReportRunStoreError(err)
	}
	parameters := make([]ReportParameterDTO, 0, len(published.Parameters))
	for _, parameter := range published.Parameters {
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
		parameters = append(parameters, item)
	}
	return &ReportRunContractDTO{
		DefinitionID: published.Definition.ID, VersionID: published.Version.ID,
		Code: published.Definition.Code, Name: published.Definition.Name,
		Description: published.Definition.Description, ExecutionMode: published.Version.ExecutionMode,
		InputSchema: cloneJSON([]byte(published.Version.InputSchemaJSON)), Parameters: parameters,
	}, nil
}

func (service *ReportRunService) Create(ctx context.Context, actor, definitionID uint, request requestbody.ReportRunCreateRequest) (*ReportRunDTO, error) {
	if service == nil || service.store == nil || service.cipher == nil || ctx == nil || actor == 0 || definitionID == 0 ||
		(request.RefreshNonce != "" && !refreshNoncePattern.MatchString(request.RefreshNonce)) || len(request.Parameters) > maxReportParameters || len(request.Conditions) > maxReportParameters {
		return nil, fmt.Errorf("%w: actor, report and parameters are required", ErrReportRunInvalid)
	}
	published, err := service.store.FindPublishedReport(ctx, actor, definitionID, reportrepo.ReportActionQuery)
	if err != nil {
		return nil, classifyReportRunStoreError(err)
	}
	if isJSONInputReport(published.Version) {
		return service.createJSONInputRun(ctx, actor, definitionID, request, published)
	}
	definitions := reportParameterDefinitions(published.Parameters)
	runUUID := uuid.NewString()
	systemValues, err := reportSystemValues(definitions, runUUID, actor)
	if err != nil {
		return nil, fmt.Errorf("%w: published system parameters are invalid", ErrReportRunInvalid)
	}
	normalized, err := reporting.NormalizeParameters(definitions, cloneParameterInput(request.Parameters), systemValues)
	if err != nil {
		if errors.Is(err, reporting.ErrInvalidParameterInput) || errors.Is(err, reporting.ErrInvalidParameterContract) {
			return nil, fmt.Errorf("%w: parameters do not match the published contract", ErrReportRunInvalid)
		}
		return nil, fmt.Errorf("report run service: normalize parameters: %w", err)
	}
	keyVersion, sensitiveCipher, err := encryptSensitiveParameters(service.cipher, normalized.SensitiveJSON)
	if err != nil {
		if errors.Is(err, reportsecret.ErrInvalidCredential) {
			return nil, fmt.Errorf("%w: %v", ErrReportRunCredentialUnavailable, err)
		}
		return nil, fmt.Errorf("report run service: encrypt sensitive parameters: %w", err)
	}
	createdAt := service.now().UTC()
	command := &reportrepo.CreateRunCommand{
		Run: model.ReportRun{
			RunUUID: runUUID, DefinitionID: definitionID, VersionID: published.Version.ID, RequestedBy: actor,
			Status: model.ReportRunStatusQueued, ExecutionFingerprint: executionFingerprint(published.Version.ContractHash, normalized.Fingerprint, request.RefreshNonce),
			RefreshNonce: request.RefreshNonce, NormalizedParametersJSON: model.JSONText(normalized.PublicJSON),
			SensitiveParametersCipher: sensitiveCipher, SensitiveParametersKeyVersion: keyVersion,
			ContractHash: published.Version.ContractHash, ProcedureSignatureHash: published.Version.ProcedureSignatureHash,
			ResultSchemaHash: published.Version.ResultSchemaHash,
		},
		Outbox: reportrepo.NewReportRunOutbox(runUUID, createdAt),
	}
	if err := service.store.CreateRun(ctx, actor, definitionID, command); err != nil {
		return nil, classifyReportRunStoreError(err)
	}
	return &ReportRunDTO{
		ID: command.Run.ID, RunUUID: command.Run.RunUUID, DefinitionID: definitionID,
		VersionID: command.Run.VersionID, Status: command.Run.Status, CreatedAt: command.Run.CreatedAt,
	}, nil
}

type reportRunInputFieldSchema struct {
	Type          string            `json:"type"`
	DisplayName   string            `json:"displayName"`
	Control       string            `json:"control,omitempty"`
	Required      bool              `json:"required,omitempty"`
	Multiple      bool              `json:"multiple,omitempty"`
	Example       json.RawMessage   `json:"example,omitempty"`
	DefaultValue  json.RawMessage   `json:"default,omitempty"`
	AllowedValues []json.RawMessage `json:"allowedValues,omitempty"`
}

func (service *ReportRunService) createJSONInputRun(
	ctx context.Context,
	actor, definitionID uint,
	request requestbody.ReportRunCreateRequest,
	published *reportrepo.PublishedReport,
) (*ReportRunDTO, error) {
	if len(request.Parameters) > 0 && len(request.Conditions) > 0 {
		return nil, fmt.Errorf("%w: use conditions for JSON input reports", ErrReportRunInvalid)
	}
	conditions := request.Conditions
	if conditions == nil {
		conditions = request.Parameters
	}
	canonical, fingerprint, err := normalizeRefCursorConditions([]byte(published.Version.InputSchemaJSON), conditions)
	if err != nil {
		return nil, fmt.Errorf("%w: conditions do not match the published JSON schema", ErrReportRunInvalid)
	}
	runUUID := uuid.NewString()
	createdAt := service.now().UTC()
	command := &reportrepo.CreateRunCommand{
		Run: model.ReportRun{
			RunUUID: runUUID, DefinitionID: definitionID, VersionID: published.Version.ID, RequestedBy: actor,
			Status: model.ReportRunStatusQueued, ExecutionFingerprint: executionFingerprint(published.Version.ContractHash, fingerprint, request.RefreshNonce),
			RefreshNonce: request.RefreshNonce, NormalizedParametersJSON: model.JSONText(canonical),
			ContractHash: published.Version.ContractHash, ProcedureSignatureHash: published.Version.ProcedureSignatureHash,
			ResultSchemaHash: published.Version.ResultSchemaHash,
		},
		Outbox: reportrepo.NewReportRunOutbox(runUUID, createdAt),
	}
	if err := service.store.CreateRun(ctx, actor, definitionID, command); err != nil {
		return nil, classifyReportRunStoreError(err)
	}
	return &ReportRunDTO{
		ID: command.Run.ID, RunUUID: command.Run.RunUUID, DefinitionID: definitionID,
		VersionID: command.Run.VersionID, Status: command.Run.Status, CreatedAt: command.Run.CreatedAt,
	}, nil
}

func normalizeRefCursorConditions(schemaJSON []byte, input map[string]json.RawMessage) ([]byte, string, error) {
	totalBytes := 0
	for code, value := range input {
		totalBytes += len(code) + len(value)
		if totalBytes > maxReportJSONConditionsBytes {
			return nil, "", ErrReportRunInvalid
		}
	}
	var schema map[string]reportRunInputFieldSchema
	decoder := json.NewDecoder(bytes.NewReader(schemaJSON))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&schema); err != nil || len(schema) == 0 || len(schema) > maxReportParameters {
		return nil, "", ErrReportRunInvalid
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, "", ErrReportRunInvalid
	}
	for code := range input {
		if _, ok := schema[code]; !ok {
			return nil, "", ErrReportRunInvalid
		}
	}
	normalized := make(map[string]json.RawMessage, len(schema))
	for code, field := range schema {
		value, supplied := input[code]
		if !supplied || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			if len(bytes.TrimSpace(field.DefaultValue)) > 0 {
				value, supplied = field.DefaultValue, true
			} else if field.Required {
				return nil, "", ErrReportRunInvalid
			} else {
				continue
			}
		}
		canonical, decoded, err := canonicalConditionValue(value)
		if err != nil || !conditionValueMatchesField(decoded, field) || !conditionValueAllowed(canonical, field.AllowedValues, field.Multiple) {
			return nil, "", ErrReportRunInvalid
		}
		normalized[code] = canonical
	}
	encoded, err := json.Marshal(normalized)
	if err != nil {
		return nil, "", err
	}
	if len(encoded) > maxReportJSONConditionsBytes {
		return nil, "", ErrReportRunInvalid
	}
	sum := sha256.Sum256(encoded)
	return encoded, hex.EncodeToString(sum[:]), nil
}

func canonicalConditionValue(raw json.RawMessage) (json.RawMessage, interface{}, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value interface{}
	if err := decoder.Decode(&value); err != nil {
		return nil, nil, err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, nil, ErrReportRunInvalid
	}
	encoded, err := json.Marshal(value)
	return encoded, value, err
}

func conditionValueMatchesField(value interface{}, field reportRunInputFieldSchema) bool {
	if field.Multiple {
		values, ok := value.([]interface{})
		if !ok || (field.Required && len(values) == 0) {
			return false
		}
		for _, item := range values {
			if !conditionScalarMatchesType(item, field.Type) {
				return false
			}
		}
		return true
	}
	if _, isArray := value.([]interface{}); isArray {
		return false
	}
	return conditionScalarMatchesType(value, field.Type)
}

func conditionScalarMatchesType(value interface{}, oracleType string) bool {
	switch strings.ToUpper(strings.TrimSpace(oracleType)) {
	case "NUMBER":
		switch typed := value.(type) {
		case json.Number:
			return typed.String() != ""
		case string:
			_, err := strconv.ParseFloat(typed, 64)
			return err == nil
		default:
			return false
		}
	case "BOOLEAN":
		_, ok := value.(bool)
		return ok
	case "VARCHAR2", "NVARCHAR2", "CHAR", "NCHAR", "CLOB", "NCLOB", "DATE", "TIMESTAMP":
		text, ok := value.(string)
		return ok && text != ""
	default:
		return false
	}
}

func conditionValueAllowed(value json.RawMessage, allowed []json.RawMessage, multiple bool) bool {
	if len(allowed) == 0 {
		return true
	}
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, item := range allowed {
		canonical, _, err := canonicalConditionValue(item)
		if err != nil {
			return false
		}
		allowedSet[string(canonical)] = struct{}{}
	}
	if !multiple {
		_, ok := allowedSet[string(value)]
		return ok
	}
	var items []json.RawMessage
	if err := json.Unmarshal(value, &items); err != nil {
		return false
	}
	for _, item := range items {
		canonical, _, err := canonicalConditionValue(item)
		if err != nil {
			return false
		}
		if _, ok := allowedSet[string(canonical)]; !ok {
			return false
		}
	}
	return true
}

func reportParameterDefinitions(parameters []model.ReportParameter) []reporting.ParameterDefinition {
	definitions := make([]reporting.ParameterDefinition, 0, len(parameters))
	for _, parameter := range parameters {
		definitions = append(definitions, reporting.ParameterDefinition{
			Code: parameter.ParameterCode, ProcedureArgName: parameter.ProcedureArgName, Position: parameter.Position,
			Direction: parameter.Direction, LogicalType: parameter.LogicalType, OracleType: parameter.OracleType,
			Cardinality: parameter.Cardinality, Required: parameter.Required, Nullable: parameter.Nullable,
			SystemInjected: parameter.SystemInjected, Sensitive: parameter.Sensitive,
			DefaultValue: json.RawMessage(parameter.DefaultValueJSON), AllowedValues: json.RawMessage(parameter.AllowedValuesJSON),
			Validation: json.RawMessage(parameter.ValidationJSON), Normalizer: json.RawMessage(parameter.NormalizerJSON),
			ValueSource: json.RawMessage(parameter.ValueSourceJSON), Timezone: parameter.Timezone,
			NullPolicy: parameter.NullPolicy, CollectionEncoding: parameter.CollectionEncoding,
		})
	}
	return definitions
}

func reportSystemValues(definitions []reporting.ParameterDefinition, runUUID string, actor uint) (map[string]interface{}, error) {
	values := make(map[string]interface{})
	for _, definition := range definitions {
		source, err := reporting.SystemValueSource(definition)
		if err != nil {
			return nil, err
		}
		switch source {
		case reporting.ValueSourceRunID:
			values[definition.Code] = runUUID
		case reporting.ValueSourceActorID:
			values[definition.Code] = actor
		}
	}
	return values, nil
}

func cloneParameterInput(input map[string]json.RawMessage) map[string]json.RawMessage {
	result := make(map[string]json.RawMessage, len(input))
	for key, value := range input {
		result[key] = append(json.RawMessage(nil), value...)
	}
	return result
}

func encryptSensitiveParameters(cipher reportParameterCipher, sensitiveJSON []byte) (string, string, error) {
	if bytes.Equal(bytes.TrimSpace(sensitiveJSON), []byte(`{}`)) {
		return "", "", nil
	}
	return cipher.Encrypt(sensitiveJSON)
}

func executionFingerprint(contractHash, parameterFingerprint, refreshNonce string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(contractHash) + "\x1f" + parameterFingerprint + "\x1f" + strings.TrimSpace(refreshNonce)))
	return hex.EncodeToString(sum[:])
}

func classifyReportRunStoreError(err error) error {
	switch {
	case errors.Is(err, reportrepo.ErrPublishedReportNotFound):
		return ErrReportNotFound
	case errors.Is(err, reportrepo.ErrReportActionDenied):
		return ErrReportRunDenied
	case errors.Is(err, reportrepo.ErrDraftVersionConflict):
		return ErrReportConflict
	case errors.Is(err, reportrepo.ErrInvalidRun):
		return fmt.Errorf("%w: repository rejected run", ErrReportRunInvalid)
	default:
		return fmt.Errorf("report run service: store: %w", err)
	}
}
