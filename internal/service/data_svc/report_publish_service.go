package data_svc

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"gin-biz-web-api/internal/reportcontract"
	"gin-biz-web-api/internal/reportoracle"
	"gin-biz-web-api/internal/reportrepo"
	"gin-biz-web-api/model"
)

var ErrReportPublicationInvalid = errors.New("report publication: invalid contract")

const defaultReportPublicationInspectionTimeout = 5 * time.Minute

type reportPublicationStore interface {
	FindDraftByID(context.Context, uint, uint) (*reportrepo.Draft, error)
	FindDatasource(context.Context, uint) (*model.ReportDatasource, error)
	PublishDraft(context.Context, uint, uint, uint64, reportrepo.Publication) (*reportrepo.Draft, error)
}

type reportCredentialDecryptor interface {
	Decrypt(version, ciphertext string) (string, error)
}

type reportOracleInspector interface {
	InspectProcedure(context.Context, reportoracle.ProcedureRef) ([]reportoracle.ProcedureArgument, error)
	InspectResultTable(context.Context, reportoracle.ResultTableRef) ([]reportoracle.ResultColumn, error)
	InspectResultSnapshotContract(context.Context, reportoracle.ResultSnapshotRef) (reportoracle.ResultSnapshotContract, error)
	Close() error
}

type reportOracleOpener func(context.Context, reportoracle.Config) (reportOracleInspector, error)

func OpenReportOracle(ctx context.Context, config reportoracle.Config) (reportOracleInspector, error) {
	return reportoracle.Open(ctx, config)
}

type ReportPublishService struct {
	store     reportPublicationStore
	decryptor reportCredentialDecryptor
	open      reportOracleOpener
	now       func() time.Time
}

type ReportPublicationDTO struct {
	DefinitionID uint      `json:"definitionId"`
	VersionID    uint      `json:"versionId"`
	Version      uint64    `json:"version"`
	Status       string    `json:"status"`
	ContractHash string    `json:"contractHash"`
	PublishedAt  time.Time `json:"publishedAt"`
}

func NewReportPublishService(store reportPublicationStore, decryptor reportCredentialDecryptor, opener reportOracleOpener) *ReportPublishService {
	if store == nil || decryptor == nil || opener == nil {
		panic("report publication: dependencies are required")
	}
	return &ReportPublishService{store: store, decryptor: decryptor, open: opener, now: func() time.Time { return time.Now().UTC() }}
}

func (service *ReportPublishService) Publish(ctx context.Context, actor, definitionID uint, expectedLockVersion uint64) (*ReportPublicationDTO, error) {
	if service == nil || service.store == nil || service.decryptor == nil || service.open == nil || ctx == nil || actor == 0 || definitionID == 0 || expectedLockVersion == 0 {
		return nil, fmt.Errorf("%w: service, actor, report and lock version are required", ErrReportPublicationInvalid)
	}
	draft, err := service.store.FindDraftByID(ctx, actor, definitionID)
	if err != nil {
		return nil, classifyPublicationStoreError(err)
	}
	if draft.LockVersion != expectedLockVersion {
		return nil, ErrReportConflict
	}
	if draft.Version.DatasourceID == 0 || draft.Version.DatasourceID != draft.Definition.DatasourceID {
		return nil, fmt.Errorf("%w: datasource snapshot is inconsistent", ErrReportPublicationInvalid)
	}
	datasource, err := service.store.FindDatasource(ctx, draft.Version.DatasourceID)
	if err != nil {
		return nil, classifyPublicationStoreError(err)
	}
	password, err := service.decryptor.Decrypt(datasource.CredentialKeyVersion, datasource.PasswordCiphertext)
	if err != nil {
		return nil, fmt.Errorf("report publication: decrypt datasource credential: %w", err)
	}
	compiled, err := service.inspectContract(ctx, datasource, password, draft)
	if err != nil {
		return nil, err
	}
	validatedAt := service.now().UTC()
	published, err := service.store.PublishDraft(ctx, actor, definitionID, expectedLockVersion, reportrepo.Publication{
		CompiledSpecJSON: model.JSONText(compiled.SpecJSON), ContractHash: compiled.Hashes.Contract,
		ParameterSchemaHash: compiled.Hashes.ParameterSchema, ProcedureSignatureHash: compiled.Hashes.ProcedureSignature,
		ResultSchemaHash: compiled.Hashes.ResultSchema, PermissionHash: compiled.Hashes.Permission,
		ExportSchemaHash: compiled.Hashes.ExportSchema, SchemaProbeToken: uuid.NewString(), SchemaValidatedAt: validatedAt,
	})
	if err != nil {
		return nil, classifyPublicationStoreError(err)
	}
	publishedAt := validatedAt
	if published.Version.PublishedAt != nil {
		publishedAt = published.Version.PublishedAt.UTC()
	}
	return &ReportPublicationDTO{
		DefinitionID: published.Definition.ID, VersionID: published.Version.ID, Version: published.Version.VersionNumber,
		Status: model.ReportVersionStatusPublished, ContractHash: compiled.Hashes.Contract, PublishedAt: publishedAt,
	}, nil
}

func (service *ReportPublishService) inspectContract(
	ctx context.Context,
	datasource *model.ReportDatasource,
	password string,
	draft *reportrepo.Draft,
) (compiled reportcontract.Compiled, resultErr error) {
	timeout := defaultReportPublicationInspectionTimeout
	if datasource.QueryTimeoutSeconds > 0 {
		timeout = time.Duration(datasource.QueryTimeoutSeconds) * time.Second
	}
	inspectionCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	inspector, err := service.open(inspectionCtx, oracleConfigFromDatasource(*datasource, password))
	if err != nil {
		return compiled, fmt.Errorf("report publication: open oracle datasource: %w", err)
	}
	defer func() {
		if closeErr := inspector.Close(); closeErr != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("report publication: close oracle datasource: %w", closeErr))
		}
	}()

	procedureRef := reportoracle.ProcedureRef{Owner: draft.Version.ProcedureOwner, Package: draft.Version.PackageName, Name: draft.Version.ProcedureName, Overload: draft.Version.ProcedureOverload}
	procedure, err := inspector.InspectProcedure(inspectionCtx, procedureRef)
	if err != nil {
		return compiled, classifyOracleInspectionError("procedure", err)
	}
	resultRef := reportoracle.ResultTableRef{Owner: draft.Version.ResultTableOwner, Name: draft.Version.ResultTableName}
	resultColumns, err := inspector.InspectResultTable(inspectionCtx, resultRef)
	if err != nil {
		return compiled, classifyOracleInspectionError("result table", err)
	}
	configuredColumns := make([]string, 0, len(draft.Columns))
	for _, column := range draft.Columns {
		configuredColumns = append(configuredColumns, column.DatabaseColumn)
	}
	snapshot, err := inspector.InspectResultSnapshotContract(inspectionCtx, reportoracle.ResultSnapshotRef{
		Table: resultRef, RunIDColumn: draft.Version.ResultRunIDColumn, RowIDColumn: draft.Version.ResultRowIDColumn, Columns: configuredColumns,
	})
	if err != nil {
		return compiled, classifyOracleInspectionError("result snapshot", err)
	}
	compiled, err = reportcontract.Compile(draft.Version, draft.Parameters, draft.Columns, draft.Grants, procedure, resultColumns, snapshot)
	if err != nil {
		return compiled, fmt.Errorf("%w: %v", ErrReportPublicationInvalid, err)
	}
	return compiled, nil
}

func classifyOracleInspectionError(target string, err error) error {
	if errors.Is(err, reportoracle.ErrInvalidConfiguration) || errors.Is(err, reportoracle.ErrMetadataMismatch) || errors.Is(err, reportoracle.ErrUnsupportedBinding) {
		return fmt.Errorf("%w: inspect Oracle %s: %v", ErrReportPublicationInvalid, target, err)
	}
	return fmt.Errorf("report publication: inspect Oracle %s: %w", target, err)
}

func oracleConfigFromDatasource(datasource model.ReportDatasource, password string) reportoracle.Config {
	return reportoracle.Config{
		Host: datasource.Host, Port: datasource.Port, ServiceName: datasource.ServiceName, SID: datasource.SID,
		Username: datasource.Username, Password: password, Timezone: datasource.SessionTimezone,
		ConnectTimeout:     time.Duration(datasource.ConnectTimeoutSeconds) * time.Second,
		MaxOpenConnections: datasource.MaxOpenConnections, MaxIdleConnections: datasource.MaxIdleConnections,
		PrefetchRows: datasource.PrefetchRows, FetchArraySize: datasource.ArraySize,
	}
}

func reportOracleQueryContext(parent context.Context, datasource model.ReportDatasource) (context.Context, context.CancelFunc) {
	timeout := time.Duration(datasource.QueryTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = defaultReportPublicationInspectionTimeout
	}
	return context.WithTimeout(parent, timeout)
}

func classifyPublicationStoreError(err error) error {
	switch {
	case errors.Is(err, reportrepo.ErrDraftNotFound):
		return ErrReportNotFound
	case errors.Is(err, reportrepo.ErrDraftVersionConflict):
		return ErrReportConflict
	case errors.Is(err, reportrepo.ErrDatasourceUnavailable):
		return fmt.Errorf("%w: datasource does not exist, is disabled or is not Oracle", ErrReportPublicationInvalid)
	case errors.Is(err, reportrepo.ErrInvalidDraft):
		return fmt.Errorf("%w: repository rejected publication", ErrReportPublicationInvalid)
	default:
		return fmt.Errorf("report publication: store: %w", err)
	}
}
