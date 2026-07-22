package data_svc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"

	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/job"
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/database"
	"gin-biz-web-api/pkg/storage"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	mallWeatherExportOperationScope = "weather.export"
	mallWeatherExportLimitConfigKey = "mall_weather_export_limits"

	defaultMallWeatherExportMaxEstimatedRows = int64(2_000_000)
	defaultMallWeatherExportMaxRangeDays     = 366
	defaultMallWeatherExportEstimateTimeout  = 10 * time.Second
	maxMallWeatherExportConfiguredRows       = int64(20_000_000)
	maxMallWeatherExportConfiguredRangeDays  = 3_660
	defaultMallWeatherExportDownloadTTL      = 5 * time.Minute
)

var (
	ErrMallWeatherExportInvalid  = errors.New("mall weather export: invalid input")
	ErrMallWeatherExportTooLarge = errors.New("mall weather export: estimated rows exceed limit")
	ErrMallWeatherExportNotReady = errors.New("mall weather export: result is not ready")
	ErrMallWeatherExportExpired  = errors.New("mall weather export: result expired")
)

type MallWeatherExportCreateResult struct {
	JobID          string    `json:"jobId"`
	Status         string    `json:"status"`
	ProfileID      uint      `json:"profileId"`
	ProfileVersion uint64    `json:"profileVersion"`
	EstimatedRows  int64     `json:"estimatedRows"`
	CreatedBy      uint      `json:"createdBy"`
	CreatedAt      time.Time `json:"createdAt"`
}

type MallWeatherExportJobDTO struct {
	JobID            string     `json:"jobId"`
	ProfileID        uint       `json:"profileId"`
	ProfileVersion   uint64     `json:"profileVersion"`
	Status           string     `json:"status"`
	TotalRows        int64      `json:"totalRows"`
	ProcessedRows    int64      `json:"processedRows"`
	CurrentSheet     string     `json:"currentSheet,omitempty"`
	CancelRequested  bool       `json:"cancelRequested"`
	ResultChecksum   string     `json:"resultChecksum,omitempty"`
	FileSizeBytes    int64      `json:"fileSizeBytes"`
	ErrorMessageSafe string     `json:"errorMessageSafe,omitempty"`
	StartedAt        *time.Time `json:"startedAt,omitempty"`
	FinishedAt       *time.Time `json:"finishedAt,omitempty"`
	ExpiresAt        *time.Time `json:"expiresAt,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

type MallWeatherExportDownloadResult struct {
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type MallWeatherExportProfileSnapshot struct {
	ProfileID uint                           `json:"profileId"`
	Code      string                         `json:"code"`
	Name      string                         `json:"name"`
	Version   uint64                         `json:"version"`
	Config    MallWeatherExportProfileConfig `json:"config"`
}

type mallWeatherExportLimits struct {
	MaxEstimatedRows       int64 `json:"maxEstimatedRows"`
	MaxRangeDays           int   `json:"maxRangeDays"`
	EstimateTimeoutSeconds int   `json:"estimateTimeoutSeconds"`
}

type mallWeatherExportProfileReader interface {
	FindByID(context.Context, uint) (*model.MallWeatherExportProfile, error)
}

type mallWeatherExportEstimator interface {
	EstimateRows(context.Context, data_dao.MallWeatherExportEstimateRequest) (int64, error)
}

type mallWeatherExportJobReader interface {
	FindByUUIDAndActor(context.Context, string, uint) (*model.MallWeatherExportJob, error)
}

type mallWeatherExportLimitReader interface {
	GetValue(context.Context, string) (string, bool, error)
}

type mallWeatherExportDownloadSigner interface {
	PresignDownloadURL(context.Context, string, string, time.Duration) (string, error)
}

type mallWeatherExportDownloadSignerFactory func() (mallWeatherExportDownloadSigner, error)

type mallWeatherExportCreateCommand struct {
	ActorUserID         uint
	ProfileID           uint
	ProfileVersion      uint64
	ProfileCode         string
	ProfileName         string
	ProfileJSON         model.JSONText
	ProfileSnapshotJSON model.JSONText
	FiltersJSON         model.JSONText
	KeyHash             string
	JobIdempotencyHash  string
	RequestHash         string
	JobUUID             string
	EstimatedRows       int64
	RequestedAt         time.Time
}

type mallWeatherExportJobStore interface {
	Create(context.Context, mallWeatherExportCreateCommand) (*MallWeatherExportCreateResult, bool, error)
}

type gormMallWeatherExportJobStore struct {
	db *gorm.DB
}

type MallWeatherExportJobService struct {
	profiles    mallWeatherExportProfileReader
	permissions mallPermissionChecker
	estimator   mallWeatherExportEstimator
	jobs        mallWeatherExportJobReader
	limits      mallWeatherExportLimitReader
	store       mallWeatherExportJobStore
	newSigner   mallWeatherExportDownloadSignerFactory
	downloadTTL time.Duration
	now         func() time.Time
}

func NewMallWeatherExportJobService() *MallWeatherExportJobService {
	return &MallWeatherExportJobService{
		profiles:    data_dao.NewMallWeatherExportProfileDAO(database.DB),
		permissions: data_dao.NewMallWeatherPermissionDAO(database.DB),
		estimator:   data_dao.NewMallWeatherExportJobDAO(database.DB),
		jobs:        data_dao.NewMallWeatherExportJobDAO(database.DB),
		limits:      data_dao.NewRuntimeConfigDAO(),
		store:       gormMallWeatherExportJobStore{db: database.DB},
		newSigner: func() (mallWeatherExportDownloadSigner, error) {
			return storage.NewOSSClientFromConfig()
		},
		downloadTTL: defaultMallWeatherExportDownloadTTL,
		now:         time.Now,
	}
}

func newMallWeatherExportJobService(
	profiles mallWeatherExportProfileReader,
	permissions mallPermissionChecker,
	estimator mallWeatherExportEstimator,
	jobs mallWeatherExportJobReader,
	limits mallWeatherExportLimitReader,
	store mallWeatherExportJobStore,
	now func() time.Time,
) (*MallWeatherExportJobService, error) {
	if profiles == nil || permissions == nil || estimator == nil || jobs == nil || limits == nil || store == nil || now == nil {
		return nil, fmt.Errorf("mall weather export: invalid service configuration")
	}
	return &MallWeatherExportJobService{
		profiles: profiles, permissions: permissions, estimator: estimator, jobs: jobs,
		limits: limits, store: store,
		newSigner: func() (mallWeatherExportDownloadSigner, error) {
			return storage.NewOSSClientFromConfig()
		},
		downloadTTL: defaultMallWeatherExportDownloadTTL,
		now:         now,
	}, nil
}

func (service *MallWeatherExportJobService) Get(
	ctx context.Context,
	actorUserID uint,
	jobUUID string,
) (*MallWeatherExportJobDTO, error) {
	jobUUID = strings.TrimSpace(jobUUID)
	if service == nil || ctx == nil || actorUserID == 0 || len(jobUUID) != 36 || uuid.Validate(jobUUID) != nil {
		return nil, fmt.Errorf("%w: invalid job id", ErrMallWeatherExportInvalid)
	}
	if err := service.authorize(ctx, actorUserID); err != nil {
		return nil, err
	}
	row, err := service.jobs.FindByUUIDAndActor(ctx, jobUUID, actorUserID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, fmt.Errorf("mall weather export: nil stored job")
	}
	dto, err := mallWeatherExportJobDTO(row)
	if err != nil {
		return nil, err
	}
	return &dto, nil
}

func (service *MallWeatherExportJobService) Download(
	ctx context.Context,
	actorUserID uint,
	jobUUID string,
) (*MallWeatherExportDownloadResult, error) {
	jobUUID = strings.TrimSpace(jobUUID)
	if service == nil || service.newSigner == nil || service.downloadTTL < time.Minute ||
		service.downloadTTL > time.Hour || ctx == nil || actorUserID == 0 ||
		len(jobUUID) != 36 || uuid.Validate(jobUUID) != nil {
		return nil, fmt.Errorf("%w: invalid download request", ErrMallWeatherExportInvalid)
	}
	if err := service.authorize(ctx, actorUserID); err != nil {
		return nil, err
	}
	row, err := service.jobs.FindByUUIDAndActor(ctx, jobUUID, actorUserID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, fmt.Errorf("mall weather export: nil stored job")
	}
	now := service.now().UTC()
	if strings.ToLower(strings.TrimSpace(row.Status)) != "succeeded" {
		return nil, ErrMallWeatherExportNotReady
	}
	if row.ExpiresAt == nil || !row.ExpiresAt.UTC().After(now) || !validMallWeatherExportResultObjectKey(row.ResultObjectKey) {
		return nil, ErrMallWeatherExportExpired
	}
	validFor := service.downloadTTL
	if remaining := row.ExpiresAt.UTC().Sub(now); remaining < validFor {
		validFor = remaining
	}
	if validFor < time.Minute {
		return nil, ErrMallWeatherExportExpired
	}
	signer, err := service.newSigner()
	if err != nil {
		return nil, fmt.Errorf("mall weather export: create download signer: %w", err)
	}
	if signer == nil {
		return nil, fmt.Errorf("mall weather export: nil download signer")
	}
	fileName := "mall_weather_export_" + jobUUID + ".xlsx"
	url, err := signer.PresignDownloadURL(ctx, row.ResultObjectKey, fileName, validFor)
	if err != nil {
		return nil, fmt.Errorf("mall weather export: sign download: %w", err)
	}
	return &MallWeatherExportDownloadResult{URL: url, ExpiresAt: now.Add(validFor)}, nil
}

func validMallWeatherExportResultObjectKey(value string) bool {
	if value == "" || len(value) > 1024 || value != strings.TrimSpace(value) || strings.HasPrefix(value, "/") ||
		strings.Contains(value, "\\") {
		return false
	}
	for _, char := range value {
		if unicode.IsControl(char) {
			return false
		}
	}
	for _, part := range strings.Split(value, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}

func mallWeatherExportJobDTO(row *model.MallWeatherExportJob) (MallWeatherExportJobDTO, error) {
	if row == nil || row.ID == 0 || len(row.JobUUID) != 36 || uuid.Validate(row.JobUUID) != nil ||
		row.ProfileID == 0 || row.ProfileVersion == 0 || row.TotalRows < 0 || row.ProcessedRows < 0 ||
		row.FileSizeBytes < 0 || row.CreatedAt.IsZero() || row.UpdatedAt.IsZero() {
		return MallWeatherExportJobDTO{}, fmt.Errorf("mall weather export: invalid stored job")
	}
	status := strings.ToLower(strings.TrimSpace(row.Status))
	if !mallWeatherExportJobStatuses[status] {
		return MallWeatherExportJobDTO{}, fmt.Errorf("mall weather export: invalid stored status")
	}
	if row.ResultChecksum != "" {
		checksum, err := hex.DecodeString(row.ResultChecksum)
		if err != nil || len(checksum) != sha256.Size {
			return MallWeatherExportJobDTO{}, fmt.Errorf("mall weather export: invalid stored checksum")
		}
	}
	return MallWeatherExportJobDTO{
		JobID: row.JobUUID, ProfileID: row.ProfileID, ProfileVersion: row.ProfileVersion,
		Status: strings.ToUpper(status), TotalRows: row.TotalRows, ProcessedRows: row.ProcessedRows,
		CurrentSheet: row.CurrentSheet, CancelRequested: row.CancelRequested,
		ResultChecksum: row.ResultChecksum, FileSizeBytes: row.FileSizeBytes,
		ErrorMessageSafe: row.ErrorMessageSafe,
		StartedAt:        utcTimePointer(row.StartedAt), FinishedAt: utcTimePointer(row.FinishedAt),
		ExpiresAt: utcTimePointer(row.ExpiresAt), CreatedAt: row.CreatedAt.UTC(), UpdatedAt: row.UpdatedAt.UTC(),
	}, nil
}

func utcTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	utc := value.UTC()
	return &utc
}

var mallWeatherExportJobStatuses = map[string]bool{
	"pending":   true,
	"running":   true,
	"succeeded": true,
	"failed":    true,
	"cancelled": true,
	"expired":   true,
}

func (service *MallWeatherExportJobService) Create(
	ctx context.Context,
	actorUserID uint,
	idempotencyKey string,
	request requestbody.MallWeatherExportCreateRequest,
) (*MallWeatherExportCreateResult, bool, error) {
	if service == nil || ctx == nil || actorUserID == 0 || request.ProfileID == 0 {
		return nil, false, fmt.Errorf("%w: invalid create request", ErrMallWeatherExportInvalid)
	}
	if !validIdempotencyKey(idempotencyKey) {
		return nil, false, fmt.Errorf("%w: idempotency key is required", ErrMallWeatherExportInvalid)
	}
	if request.ExpectedProfileVersion != nil && *request.ExpectedProfileVersion == 0 {
		return nil, false, fmt.Errorf("%w: invalid profile version", ErrMallWeatherExportInvalid)
	}
	if err := service.authorize(ctx, actorUserID); err != nil {
		return nil, false, err
	}
	profile, err := service.profiles.FindByID(ctx, request.ProfileID)
	if err != nil {
		return nil, false, err
	}
	if !profile.Enabled {
		return nil, false, fmt.Errorf("%w: export profile is disabled", ErrMallWeatherExportInvalid)
	}
	if request.ExpectedProfileVersion != nil && *request.ExpectedProfileVersion != profile.Version {
		return nil, false, ErrMallWeatherExportProfileConflict
	}
	profileDTO, err := mallWeatherExportProfileDTO(profile)
	if err != nil {
		return nil, false, err
	}
	effectiveFilters := profileDTO.Filters
	if request.Filters != nil {
		effectiveFilters, err = normalizeMallWeatherExportFilters(*request.Filters)
		if err != nil {
			return nil, false, fmt.Errorf("%w: invalid filters", ErrMallWeatherExportInvalid)
		}
	}
	limits, err := service.loadLimits(ctx)
	if err != nil {
		return nil, false, err
	}
	if err := validateMallWeatherExportJobRange(
		profileDTO.Datasets,
		effectiveFilters,
		profileDTO.TimeZone,
		limits,
	); err != nil {
		return nil, false, err
	}
	estimateRequest, err := mallWeatherExportEstimateRequest(profileDTO, effectiveFilters, limits.MaxEstimatedRows)
	if err != nil {
		return nil, false, err
	}
	estimateTimeout := time.Duration(limits.EstimateTimeoutSeconds) * time.Second
	estimateCtx, cancel := context.WithTimeout(ctx, estimateTimeout)
	defer cancel()
	estimatedRows, err := service.estimator.EstimateRows(estimateCtx, estimateRequest)
	if err != nil {
		return nil, false, fmt.Errorf("mall weather export: estimate rows: %w", err)
	}
	if estimatedRows > limits.MaxEstimatedRows {
		return nil, false, ErrMallWeatherExportTooLarge
	}
	return service.createJob(
		ctx,
		actorUserID,
		idempotencyKey,
		request,
		profile,
		profileDTO,
		effectiveFilters,
		estimatedRows,
	)
}

func (service *MallWeatherExportJobService) authorize(ctx context.Context, actorUserID uint) error {
	allowed, err := service.permissions.HasPermission(ctx, actorUserID, PermissionWeatherExport, service.now().UTC())
	if err != nil {
		return fmt.Errorf("mall weather export: authorize: %w", err)
	}
	if !allowed {
		return ErrMallForbidden
	}
	return nil
}

func (service *MallWeatherExportJobService) loadLimits(ctx context.Context) (mallWeatherExportLimits, error) {
	limits := mallWeatherExportLimits{
		MaxEstimatedRows:       defaultMallWeatherExportMaxEstimatedRows,
		MaxRangeDays:           defaultMallWeatherExportMaxRangeDays,
		EstimateTimeoutSeconds: int(defaultMallWeatherExportEstimateTimeout / time.Second),
	}
	value, exists, err := service.limits.GetValue(ctx, mallWeatherExportLimitConfigKey)
	if err != nil {
		return limits, fmt.Errorf("mall weather export: read limits: %w", err)
	}
	if !exists {
		return limits, nil
	}
	decoder := json.NewDecoder(strings.NewReader(value))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&limits); err != nil {
		return limits, fmt.Errorf("mall weather export: decode limits: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return limits, fmt.Errorf("mall weather export: decode limits: trailing data")
	}
	invalidRows := limits.MaxEstimatedRows < 1 || limits.MaxEstimatedRows > maxMallWeatherExportConfiguredRows
	invalidRange := limits.MaxRangeDays < 1 || limits.MaxRangeDays > maxMallWeatherExportConfiguredRangeDays
	invalidTimeout := limits.EstimateTimeoutSeconds < 1 || limits.EstimateTimeoutSeconds > 60
	if invalidRows || invalidRange || invalidTimeout {
		return limits, fmt.Errorf("mall weather export: invalid runtime limits")
	}
	return limits, nil
}

func (service *MallWeatherExportJobService) createJob(
	ctx context.Context,
	actorUserID uint,
	idempotencyKey string,
	request requestbody.MallWeatherExportCreateRequest,
	profile *model.MallWeatherExportProfile,
	profileDTO MallWeatherExportProfileDTO,
	filters requestbody.MallWeatherExportFilters,
	estimatedRows int64,
) (*MallWeatherExportCreateResult, bool, error) {
	config := MallWeatherExportProfileConfig{
		TimeZone: profileDTO.TimeZone, UnitSystem: profileDTO.UnitSystem,
		DateFormat: profileDTO.DateFormat, DateTimeFormat: profileDTO.DateTimeFormat,
		FileNameTemplate: profileDTO.FileNameTemplate, Filters: profileDTO.Filters, Datasets: profileDTO.Datasets,
	}
	profileSnapshot, err := json.Marshal(MallWeatherExportProfileSnapshot{
		ProfileID: profile.ID, Code: profile.Code, Name: profile.Name, Version: profile.Version, Config: config,
	})
	if err != nil {
		return nil, false, fmt.Errorf("mall weather export: encode profile snapshot: %w", err)
	}
	filtersJSON, err := json.Marshal(filters)
	if err != nil {
		return nil, false, fmt.Errorf("mall weather export: encode filters: %w", err)
	}
	requestHash, err := hashJSON(struct {
		ProfileID              uint                                 `json:"profileId"`
		ExpectedProfileVersion *uint64                              `json:"expectedProfileVersion,omitempty"`
		Filters                requestbody.MallWeatherExportFilters `json:"filters"`
	}{ProfileID: request.ProfileID, ExpectedProfileVersion: request.ExpectedProfileVersion, Filters: filters})
	if err != nil {
		return nil, false, fmt.Errorf("mall weather export: hash request: %w", err)
	}
	keyHash := sha256Hex([]byte(idempotencyKey))
	jobIdempotencySum := sha256.Sum256([]byte(strconv.FormatUint(uint64(actorUserID), 10) + "\x1f" + keyHash))
	command := mallWeatherExportCreateCommand{
		ActorUserID: actorUserID, ProfileID: profile.ID, ProfileVersion: profile.Version,
		ProfileCode: profile.Code, ProfileName: profile.Name, ProfileJSON: profile.ProfileJSON,
		ProfileSnapshotJSON: model.JSONText(profileSnapshot), FiltersJSON: model.JSONText(filtersJSON),
		KeyHash: keyHash, JobIdempotencyHash: fmt.Sprintf("%x", jobIdempotencySum[:]), RequestHash: requestHash,
		JobUUID: uuid.NewString(), EstimatedRows: estimatedRows, RequestedAt: service.now().UTC(),
	}
	return service.store.Create(ctx, command)
}

func validateMallWeatherExportJobRange(
	datasets []requestbody.MallWeatherExportDataset,
	filters requestbody.MallWeatherExportFilters,
	timeZone string,
	limits mallWeatherExportLimits,
) error {
	requiresRange := false
	for _, dataset := range datasets {
		if dataset.Kind == "fetch_runs" || (dataset.Latest != nil && !*dataset.Latest) {
			requiresRange = true
			break
		}
	}
	if requiresRange && (filters.Start == "" || filters.End == "") {
		return fmt.Errorf("%w: historical datasets require a time range", ErrMallWeatherExportInvalid)
	}
	if filters.Start == "" || filters.End == "" {
		return nil
	}
	start, err := time.Parse(time.RFC3339Nano, filters.Start)
	if err != nil {
		return fmt.Errorf("%w: invalid start time", ErrMallWeatherExportInvalid)
	}
	end, err := time.Parse(time.RFC3339Nano, filters.End)
	if err != nil {
		return fmt.Errorf("%w: invalid end time", ErrMallWeatherExportInvalid)
	}
	if end.Sub(start) > time.Duration(limits.MaxRangeDays)*24*time.Hour {
		return fmt.Errorf("%w: export time range is too large", ErrMallWeatherExportInvalid)
	}
	if _, err := time.LoadLocation(timeZone); err != nil {
		return fmt.Errorf("mall weather export: invalid profile time zone")
	}
	return nil
}

func mallWeatherExportEstimateRequest(
	profile MallWeatherExportProfileDTO,
	filters requestbody.MallWeatherExportFilters,
	stopAfter int64,
) (data_dao.MallWeatherExportEstimateRequest, error) {
	location, err := time.LoadLocation(profile.TimeZone)
	if err != nil {
		return data_dao.MallWeatherExportEstimateRequest{}, fmt.Errorf("mall weather export: invalid profile time zone")
	}
	filter := data_dao.MallWeatherExportEstimateFilter{
		MallIDs: append([]uint(nil), filters.MallIDs...), Cities: append([]string(nil), filters.Cities...),
		MallStatuses:    append([]string(nil), filters.MallStatuses...),
		QualityStatuses: append([]string(nil), filters.QualityStatuses...),
	}
	if filters.Start != "" {
		start, _ := time.Parse(time.RFC3339Nano, filters.Start)
		start = start.UTC()
		filter.StartUTC = &start
		filter.StartDate = start.In(location).Format(time.DateOnly)
	}
	if filters.End != "" {
		end, _ := time.Parse(time.RFC3339Nano, filters.End)
		end = end.UTC()
		filter.EndUTC = &end
		filter.EndDate = end.In(location).Format(time.DateOnly)
	}
	datasets := make([]data_dao.MallWeatherExportEstimateDataset, len(profile.Datasets))
	for index, dataset := range profile.Datasets {
		datasets[index] = data_dao.MallWeatherExportEstimateDataset{Kind: dataset.Kind}
		if dataset.Latest != nil {
			datasets[index].Latest = *dataset.Latest
		}
		if dataset.AsOf != "" {
			asOf, err := time.Parse(time.RFC3339Nano, dataset.AsOf)
			if err != nil {
				return data_dao.MallWeatherExportEstimateRequest{}, fmt.Errorf("mall weather export: invalid dataset asOf")
			}
			asOf = asOf.UTC()
			datasets[index].AsOfUTC = &asOf
		}
	}
	return data_dao.MallWeatherExportEstimateRequest{Datasets: datasets, Filter: filter, StopAfter: stopAfter}, nil
}

func (store gormMallWeatherExportJobStore) Create(
	ctx context.Context,
	command mallWeatherExportCreateCommand,
) (*MallWeatherExportCreateResult, bool, error) {
	if store.db == nil || ctx == nil || !validMallWeatherExportCreateCommand(command) {
		return nil, false, fmt.Errorf("mall weather export: invalid store command")
	}
	var result *MallWeatherExportCreateResult
	var replayed bool
	err := store.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		idempotencyDAO := data_dao.NewAPIIdempotencyDAO(tx)
		record := &model.APIIdempotencyRecord{
			OperationScope: mallWeatherExportOperationScope, ActorUserID: command.ActorUserID,
			KeyHash: command.KeyHash, RequestHash: command.RequestHash, ResourceType: "weather_export_job",
			ResponseJSON: model.JSONText(`{}`),
		}
		reserved, err := idempotencyDAO.Reserve(ctx, record)
		if err != nil {
			return err
		}
		if !reserved {
			existing, err := idempotencyDAO.FindForUpdate(
				ctx, mallWeatherExportOperationScope, command.ActorUserID, command.KeyHash,
			)
			if err != nil {
				return err
			}
			if existing.RequestHash != command.RequestHash {
				return ErrMallIdempotencyConflict
			}
			if existing.ResourceID == 0 || existing.HTTPStatus == 0 ||
				existing.ResponseJSON == "" || existing.ResponseJSON == model.JSONText(`{}`) {
				return ErrMallIdempotencyPending
			}
			var snapshot MallWeatherExportCreateResult
			if err := json.Unmarshal([]byte(existing.ResponseJSON), &snapshot); err != nil {
				return fmt.Errorf("mall weather export: decode idempotency response: %w", err)
			}
			result, replayed = &snapshot, true
			return nil
		}

		var current model.MallWeatherExportProfile
		err = tx.Clauses(clause.Locking{Strength: "SHARE"}).Where("id = ?", command.ProfileID).First(&current).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return data_dao.ErrMallWeatherExportProfileNotFound
		}
		if err != nil {
			return fmt.Errorf("mall weather export: lock profile: %w", err)
		}
		profileChanged := !current.Enabled || current.Version != command.ProfileVersion ||
			current.Code != command.ProfileCode || current.Name != command.ProfileName ||
			current.ProfileJSON != command.ProfileJSON
		if profileChanged {
			return ErrMallWeatherExportProfileConflict
		}

		row := &model.MallWeatherExportJob{
			JobUUID: command.JobUUID, ProfileID: command.ProfileID, ProfileVersion: command.ProfileVersion,
			ProfileSnapshotJSON: command.ProfileSnapshotJSON, FiltersJSON: command.FiltersJSON,
			IdempotencyKey: command.JobIdempotencyHash, Status: "pending", TotalRows: command.EstimatedRows,
			CreatedBy: command.ActorUserID,
		}
		if err := tx.Create(row).Error; err != nil {
			return fmt.Errorf("mall weather export: create job: %w", err)
		}
		outbox, err := newMallWeatherExportOutbox(row.ID, row.JobUUID, command.RequestedAt)
		if err != nil {
			return err
		}
		if err := data_dao.NewAsyncJobOutboxDAO(tx).Create(ctx, &outbox); err != nil {
			return fmt.Errorf("mall weather export: create outbox: %w", err)
		}
		created := &MallWeatherExportCreateResult{
			JobID: row.JobUUID, Status: "PENDING", ProfileID: row.ProfileID,
			ProfileVersion: row.ProfileVersion, EstimatedRows: row.TotalRows,
			CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt.UTC(),
		}
		responseJSON, err := json.Marshal(created)
		if err != nil {
			return fmt.Errorf("mall weather export: encode response: %w", err)
		}
		if err := idempotencyDAO.Complete(
			ctx, record.ID, row.ID, http.StatusAccepted, model.JSONText(responseJSON),
		); err != nil {
			return err
		}
		result = created
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return result, replayed, nil
}

func validMallWeatherExportCreateCommand(command mallWeatherExportCreateCommand) bool {
	return command.ActorUserID != 0 && command.ProfileID != 0 && command.ProfileVersion != 0 &&
		command.ProfileCode != "" && command.ProfileName != "" && command.ProfileJSON != "" &&
		json.Valid([]byte(command.ProfileJSON)) && json.Valid([]byte(command.ProfileSnapshotJSON)) &&
		json.Valid([]byte(command.FiltersJSON)) &&
		len(command.KeyHash) == 64 &&
		len(command.JobIdempotencyHash) == 64 && len(command.RequestHash) == 64 &&
		len(command.JobUUID) == 36 && uuid.Validate(command.JobUUID) == nil &&
		command.EstimatedRows >= 0 && !command.RequestedAt.IsZero()
}

func newMallWeatherExportOutbox(
	exportJobID uint,
	jobUUID string,
	availableAt time.Time,
) (model.AsyncJobOutbox, error) {
	if exportJobID == 0 || len(jobUUID) != 36 || uuid.Validate(jobUUID) != nil || availableAt.IsZero() {
		return model.AsyncJobOutbox{}, fmt.Errorf("mall weather export: invalid outbox identity")
	}
	payload, err := json.Marshal(job.MallWeatherExportTaskPayload{ExportJobID: exportJobID})
	if err != nil {
		return model.AsyncJobOutbox{}, fmt.Errorf("mall weather export: encode outbox payload: %w", err)
	}
	if _, err := job.NewMallWeatherTask(job.TypeMallWeatherExport, payload); err != nil {
		return model.AsyncJobOutbox{}, fmt.Errorf("mall weather export: validate outbox payload: %w", err)
	}
	return model.AsyncJobOutbox{
		TaskKey: "mall:weather:export:" + jobUUID, TaskType: job.TypeMallWeatherExport,
		PayloadJSON: model.JSONText(payload), QueueName: job.MallExportQueueName, AvailableAt: availableAt.UTC(),
	}, nil
}
