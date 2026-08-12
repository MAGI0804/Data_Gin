package data_svc

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gin-biz-web-api/internal/reportrepo"
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/storage"
)

var (
	ErrReportExportQueryInvalid       = errors.New("report export query: invalid input")
	ErrReportExportQueryNotFound      = errors.New("report export query: not found")
	ErrReportExportQueryNotReady      = errors.New("report export query: not ready")
	ErrReportExportQueryExpired       = errors.New("report export query: expired")
	ErrReportExportArtifactMissing    = errors.New("report export query: artifact missing")
	ErrReportExportStorageUnavailable = errors.New("report export query: storage unavailable")
)

const (
	defaultReportExportDownloadTTL = 5 * time.Minute
	defaultReportExportStatTimeout = 10 * time.Second
)

type reportExportQueryStore interface {
	FindExportForActor(context.Context, uint, uint) (*model.ReportExport, error)
}

type reportExportDownloadSigner interface {
	StatDownloadObject(context.Context, string) (storage.ObjectMetadata, error)
	PresignDownloadURL(context.Context, string, string, time.Duration) (string, error)
}

type ReportExportViewDTO struct {
	ID                 uint       `json:"id"`
	ExportUUID         string     `json:"exportUuid"`
	RunID              uint       `json:"runId"`
	Status             string     `json:"status"`
	ProcessedRows      int64      `json:"processedRows"`
	ExportedRows       int64      `json:"exportedRows"`
	CurrentSheet       string     `json:"currentSheet,omitempty"`
	SheetCount         int        `json:"sheetCount"`
	TruncatedCellCount int64      `json:"truncatedCellCount"`
	FileSizeBytes      int64      `json:"fileSizeBytes"`
	CreatedAt          time.Time  `json:"createdAt"`
	StartedAt          *time.Time `json:"startedAt,omitempty"`
	ReadyAt            *time.Time `json:"readyAt,omitempty"`
	ExpiresAt          *time.Time `json:"expiresAt,omitempty"`
	PurgedAt           *time.Time `json:"purgedAt,omitempty"`
	ErrorCode          string     `json:"errorCode,omitempty"`
	ErrorMessage       string     `json:"errorMessage,omitempty"`
	CanDownload        bool       `json:"canDownload"`
}

type ReportExportDownloadDTO struct {
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type ReportExportQueryService struct {
	store       reportExportQueryStore
	newSigner   func() (reportExportDownloadSigner, error)
	now         func() time.Time
	downloadTTL time.Duration
	statTimeout time.Duration
}

func NewReportExportQueryService() *ReportExportQueryService {
	return NewReportExportQueryServiceWithDependencies(reportrepo.New(), func() (reportExportDownloadSigner, error) { return storage.NewOSSClientFromConfig() })
}

func NewReportExportQueryServiceWithDependencies(store reportExportQueryStore, signer func() (reportExportDownloadSigner, error)) *ReportExportQueryService {
	if store == nil || signer == nil {
		panic("report export query: dependencies are required")
	}
	return &ReportExportQueryService{store: store, newSigner: signer, now: func() time.Time { return time.Now().UTC() }, downloadTTL: defaultReportExportDownloadTTL, statTimeout: defaultReportExportStatTimeout}
}

func (service *ReportExportQueryService) Get(ctx context.Context, actor, exportID uint) (*ReportExportViewDTO, error) {
	row, err := service.find(ctx, actor, exportID)
	if err != nil {
		return nil, err
	}
	now := service.now()
	return &ReportExportViewDTO{
		ID: row.ID, ExportUUID: row.ExportUUID, RunID: row.RunID, Status: row.Status,
		ProcessedRows: row.ProcessedRows, ExportedRows: row.ExportedRows, CurrentSheet: row.CurrentSheet,
		SheetCount: row.SheetCount, TruncatedCellCount: row.TruncatedCellCount, FileSizeBytes: row.FileSizeBytes,
		CreatedAt: row.CreatedAt, StartedAt: row.StartedAt, ReadyAt: row.ReadyAt, ExpiresAt: row.ExpiresAt,
		PurgedAt: row.PurgedAt, ErrorCode: row.ErrorCode, ErrorMessage: row.ErrorMessageSafe,
		CanDownload: row.Status == model.ReportExportStatusReady && row.ExpiresAt != nil && now.Before(row.ExpiresAt.UTC()),
	}, nil
}

func (service *ReportExportQueryService) Download(ctx context.Context, actor, exportID uint) (*ReportExportDownloadDTO, error) {
	row, err := service.find(ctx, actor, exportID)
	if err != nil {
		return nil, err
	}
	now := service.now()
	if row.Status != model.ReportExportStatusReady || row.ReadyAt == nil {
		return nil, ErrReportExportQueryNotReady
	}
	if row.ExpiresAt == nil || !now.Before(row.ExpiresAt.UTC()) {
		return nil, ErrReportExportQueryExpired
	}
	if row.ResultObjectKey == "" || row.FileSizeBytes < 1 || row.ResultChecksum == "" {
		return nil, ErrReportExportArtifactMissing
	}
	signer, err := service.newSigner()
	if err != nil || signer == nil {
		return nil, fmt.Errorf("%w: create signer", ErrReportExportStorageUnavailable)
	}
	statCtx, cancel := context.WithTimeout(ctx, service.statTimeout)
	metadata, err := signer.StatDownloadObject(statCtx, row.ResultObjectKey)
	cancel()
	if err != nil {
		if errors.Is(err, storage.ErrOSSObjectNotFound) {
			return nil, ErrReportExportArtifactMissing
		}
		return nil, fmt.Errorf("%w: stat object", ErrReportExportStorageUnavailable)
	}
	if metadata.Size != row.FileSizeBytes {
		return nil, ErrReportExportArtifactMissing
	}
	validFor := service.downloadTTL
	if remaining := row.ExpiresAt.Sub(now); remaining < validFor {
		validFor = remaining
	}
	if validFor < time.Minute {
		return nil, ErrReportExportQueryExpired
	}
	fileName := "report-" + row.ExportUUID + ".xlsx"
	url, err := signer.PresignDownloadURL(ctx, row.ResultObjectKey, fileName, validFor)
	if err != nil {
		return nil, fmt.Errorf("%w: sign download", ErrReportExportStorageUnavailable)
	}
	return &ReportExportDownloadDTO{URL: url, ExpiresAt: now.Add(validFor)}, nil
}

func (service *ReportExportQueryService) find(ctx context.Context, actor, exportID uint) (*model.ReportExport, error) {
	if service == nil || service.store == nil || service.newSigner == nil || service.now == nil || ctx == nil || actor == 0 || exportID == 0 || service.downloadTTL < time.Minute || service.downloadTTL > time.Hour || service.statTimeout <= 0 || service.statTimeout > 30*time.Second {
		return nil, ErrReportExportQueryInvalid
	}
	row, err := service.store.FindExportForActor(ctx, actor, exportID)
	if errors.Is(err, reportrepo.ErrReportExportNotFound) {
		return nil, ErrReportExportQueryNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("report export query: store: %w", err)
	}
	return row, nil
}
