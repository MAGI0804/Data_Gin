package data_svc

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
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
	DefinitionID uint                 `json:"definitionId"`
	VersionID    uint                 `json:"versionId"`
	Code         string               `json:"code"`
	Name         string               `json:"name"`
	Description  string               `json:"description"`
	Parameters   []ReportParameterDTO `json:"parameters"`
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
		Description: published.Definition.Description, Parameters: parameters,
	}, nil
}

func (service *ReportRunService) Create(ctx context.Context, actor, definitionID uint, request requestbody.ReportRunCreateRequest) (*ReportRunDTO, error) {
	if service == nil || service.store == nil || service.cipher == nil || ctx == nil || actor == 0 || definitionID == 0 ||
		(request.RefreshNonce != "" && !refreshNoncePattern.MatchString(request.RefreshNonce)) || len(request.Parameters) > maxReportParameters {
		return nil, fmt.Errorf("%w: actor, report and parameters are required", ErrReportRunInvalid)
	}
	published, err := service.store.FindPublishedReport(ctx, actor, definitionID, reportrepo.ReportActionQuery)
	if err != nil {
		return nil, classifyReportRunStoreError(err)
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
