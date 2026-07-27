package data_svc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gin-biz-web-api/connector/geocoder"
	"gin-biz-web-api/global"
	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/job"
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/compressutil"
	"gin-biz-web-api/pkg/config"
	"gin-biz-web-api/pkg/database"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrMallGeocodeStale = errors.New("mall geocode: stale task")

type MallGeocodeProcessError struct {
	Retryable bool
	err       error
}

func (processError *MallGeocodeProcessError) Error() string {
	return "mall geocode: provider request failed"
}

func (processError *MallGeocodeProcessError) Unwrap() error {
	if processError == nil {
		return nil
	}
	return processError.err
}

type mallGeocodeStore interface {
	FindMall(ctx context.Context, mallID uint) (*model.Mall, error)
	Persist(ctx context.Context, payload job.MallGeocodeTaskPayload, mall *model.Mall, outcome mallGeocodeOutcome) error
}

type mallGeocodeOutcome struct {
	Response    *geocoder.Response
	Scores      []geocoder.CandidateScore
	ProviderErr error
	StartedAt   time.Time
	FinishedAt  time.Time
}

type MallGeocodeProcessor struct {
	provider geocoder.Geocoder
	store    mallGeocodeStore
	now      func() time.Time
}

func NewMallGeocodeProcessor() (*MallGeocodeProcessor, error) {
	client, err := geocoder.NewAmapClient(
		config.GetString("cfg.amap.base_url", "https://restapi.amap.com"),
		global.Credentials.AmapWebServiceKey(),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("mall geocode: create provider client: %w", err)
	}
	return &MallGeocodeProcessor{
		provider: client,
		store: &gormMallGeocodeStore{
			db:               database.DB,
			rawRetentionDays: normalizeRawRetentionDays(config.GetInt("cfg.mall_weather.raw_retention_days", 30)),
		},
		now: time.Now,
	}, nil
}

func newMallGeocodeProcessor(provider geocoder.Geocoder, store mallGeocodeStore, now func() time.Time) *MallGeocodeProcessor {
	return &MallGeocodeProcessor{provider: provider, store: store, now: now}
}

func (processor *MallGeocodeProcessor) Process(ctx context.Context, payload job.MallGeocodeTaskPayload) error {
	if err := validateMallGeocodeProcessor(processor); err != nil {
		return err
	}
	mall, err := processor.store.FindMall(ctx, payload.MallID)
	if err != nil {
		if errors.Is(err, data_dao.ErrMallNotFound) {
			return nil
		}
		return fmt.Errorf("mall geocode: load mall: %w", err)
	}
	if mall == nil {
		return fmt.Errorf("mall geocode: store returned nil mall")
	}
	if mall.Version != payload.MallVersion || mallAddressHash(mall) != payload.AddressHash {
		return nil
	}

	aliases, err := decodeMallAliases(mall.AliasesJSON)
	if err != nil {
		return err
	}
	startedAt := processor.now().UTC()
	response, providerErr := processor.provider.Geocode(ctx, geocoder.Request{
		Address: mallGeocodeRequestAddress(mall),
		City:    mall.City,
	})
	finishedAt := processor.now().UTC()
	if errors.Is(providerErr, context.Canceled) || errors.Is(providerErr, context.DeadlineExceeded) {
		return providerErr
	}

	var scores []geocoder.CandidateScore
	if response != nil && providerErr == nil {
		scores = geocoder.ScoreCandidates(geocoder.ScoreInput{
			Name:         mall.NameCN,
			Aliases:      aliases,
			Province:     mall.Province,
			City:         mall.City,
			District:     mall.District,
			Street:       mall.Street,
			StreetNumber: mall.StreetNumber,
			Address:      mall.AddressRaw,
		}, response.Candidates)
	}
	outcome := mallGeocodeOutcome{
		Response: response, Scores: scores, ProviderErr: providerErr,
		StartedAt: startedAt, FinishedAt: finishedAt,
	}
	if err := processor.store.Persist(ctx, payload, mall, outcome); err != nil {
		if errors.Is(err, ErrMallGeocodeStale) {
			return nil
		}
		return fmt.Errorf("mall geocode: persist outcome: %w", err)
	}
	if providerErr != nil {
		var typed *geocoder.ProviderError
		retryable := errors.As(providerErr, &typed) && typed.Retryable
		return &MallGeocodeProcessError{Retryable: retryable, err: providerErr}
	}
	return nil
}

func validateMallGeocodeProcessor(processor *MallGeocodeProcessor) error {
	if processor == nil || processor.provider == nil || processor.store == nil || processor.now == nil {
		return fmt.Errorf("mall geocode: processor is not configured")
	}
	return nil
}

func decodeMallAliases(raw model.JSONText) ([]string, error) {
	if strings.TrimSpace(string(raw)) == "" {
		return nil, nil
	}
	var aliases []string
	if err := json.Unmarshal([]byte(raw), &aliases); err != nil {
		return nil, fmt.Errorf("mall geocode: decode aliases: %w", err)
	}
	return aliases, nil
}

func mallGeocodeRequestAddress(mall *model.Mall) string {
	parts := []string{mall.Province, mall.City, mall.District, mall.AddressRaw, mall.NameCN}
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			result = append(result, part)
		}
	}
	return strings.Join(result, " ")
}

type gormMallGeocodeStore struct {
	db               *gorm.DB
	rawRetentionDays int
}

func (store *gormMallGeocodeStore) FindMall(ctx context.Context, mallID uint) (*model.Mall, error) {
	return data_dao.NewMallDAO(store.db).FindByID(ctx, mallID)
}

func (store *gormMallGeocodeStore) Persist(ctx context.Context, payload job.MallGeocodeTaskPayload, original *model.Mall, outcome mallGeocodeOutcome) error {
	if store == nil || store.db == nil || original == nil {
		return fmt.Errorf("mall geocode: persistence is not configured")
	}
	run, candidates, snapshot, autoIndex, err := store.buildPersistenceRows(payload, original, outcome)
	if err != nil {
		return err
	}
	return store.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current model.Mall
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&current, payload.MallID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrMallGeocodeStale
			}
			return fmt.Errorf("mall geocode: lock mall: %w", err)
		}
		if current.Version != payload.MallVersion || mallAddressHash(&current) != payload.AddressHash {
			return ErrMallGeocodeStale
		}

		if snapshot != nil {
			if err := data_dao.NewMallWeatherDAO(tx).CreateRawSnapshot(ctx, snapshot); err != nil {
				return err
			}
			run.RawSnapshotID = &snapshot.ID
		}
		geocodeDAO := data_dao.NewMallGeocodeDAO(tx)
		if err := geocodeDAO.CreateRun(ctx, run); err != nil {
			return err
		}
		for index := range candidates {
			candidates[index].RunID = run.ID
		}
		if err := geocodeDAO.CreateCandidates(ctx, candidates); err != nil {
			return err
		}
		return store.applyMallGeocodeOutcome(ctx, tx, payload, run, candidates, autoIndex, outcome)
	})
}

func normalizeRawRetentionDays(days int) int {
	if days <= 0 {
		return 30
	}
	if days > 3650 {
		return 3650
	}
	return days
}

func (store *gormMallGeocodeStore) buildPersistenceRows(payload job.MallGeocodeTaskPayload, mall *model.Mall, outcome mallGeocodeOutcome) (*model.MallGeocodeRun, []model.MallGeocodeCandidate, *model.ProviderRawSnapshot, int, error) {
	duration := outcome.FinishedAt.Sub(outcome.StartedAt).Milliseconds()
	if duration < 0 {
		duration = 0
	}
	run := &model.MallGeocodeRun{
		MallID: payload.MallID, RequestAddress: mallGeocodeRequestAddress(mall), RequestCity: mall.City,
		AddressHash: payload.AddressHash, Status: "failed", StartedAt: outcome.StartedAt,
		FinishedAt: &outcome.FinishedAt, DurationMS: duration,
	}
	if outcome.Response != nil {
		run.ProviderStatus = outcome.Response.ProviderStatus
		run.Infocode = outcome.Response.Infocode
		run.Info = outcome.Response.Info
		run.CandidateCount = len(outcome.Response.Candidates)
	}
	if outcome.ProviderErr != nil {
		setSafeGeocodeRunError(run, outcome.ProviderErr)
		snapshot, err := store.rawSnapshot(payload.MallID, outcome)
		return run, nil, snapshot, -1, err
	}
	if outcome.Response == nil {
		return nil, nil, nil, -1, fmt.Errorf("mall geocode: missing provider response")
	}
	if len(outcome.Scores) != len(outcome.Response.Candidates) {
		return nil, nil, nil, -1, fmt.Errorf("mall geocode: candidate score count mismatch")
	}

	autoIndex, autoConfirm := geocoder.AutoConfirmCandidate(outcome.Scores)
	if !autoConfirm {
		autoIndex = -1
	}
	candidates := make([]model.MallGeocodeCandidate, 0, len(outcome.Response.Candidates))
	for index, candidate := range outcome.Response.Candidates {
		reasons, err := json.Marshal(outcome.Scores[index].Reasons)
		if err != nil {
			return nil, nil, nil, -1, fmt.Errorf("mall geocode: encode score reasons: %w", err)
		}
		candidates = append(candidates, model.MallGeocodeCandidate{
			MallID: payload.MallID, CandidateNo: index + 1, Country: candidate.Country,
			Province: candidate.Province, City: candidate.City, Citycode: candidate.Citycode,
			District: candidate.District, Adcode: candidate.Adcode, Township: candidate.Township,
			Street: candidate.Street, StreetNumber: candidate.StreetNumber,
			FormattedAddress: candidate.FormattedAddress, Longitude: candidate.Longitude,
			Latitude: candidate.Latitude, CoordinateSystem: candidate.CoordinateSystem,
			Level: candidate.Level, ConfidenceScore: outcome.Scores[index].Score,
			ScoreReasonsJSON: model.JSONText(reasons), IsSelected: index == autoIndex,
		})
	}
	if len(candidates) == 0 {
		run.Status = "no_candidates"
	} else if autoIndex >= 0 {
		run.Status = "auto_confirmed"
	} else {
		run.Status = "review_required"
	}
	snapshot, err := store.rawSnapshot(payload.MallID, outcome)
	return run, candidates, snapshot, autoIndex, err
}

func (store *gormMallGeocodeStore) rawSnapshot(mallID uint, outcome mallGeocodeOutcome) (*model.ProviderRawSnapshot, error) {
	if outcome.Response == nil || len(outcome.Response.RawJSON) == 0 {
		return nil, nil
	}
	compressed, err := compressutil.Gzip(outcome.Response.RawJSON)
	if err != nil {
		return nil, err
	}
	checksum := sha256.Sum256(outcome.Response.RawJSON)
	expiresAt := outcome.FinishedAt.Add(time.Duration(store.rawRetentionDays) * 24 * time.Hour)
	return &model.ProviderRawSnapshot{
		Provider: "amap", EndpointKind: "geocode", MallID: &mallID,
		ResponseChecksum: hex.EncodeToString(checksum[:]), Compression: "gzip",
		ContentBlob: compressed, ContentLength: int64(len(outcome.Response.RawJSON)),
		SchemaVersion: "amap-geocode-v1", ExpiresAt: &expiresAt,
	}, nil
}

func (store *gormMallGeocodeStore) applyMallGeocodeOutcome(ctx context.Context, tx *gorm.DB, payload job.MallGeocodeTaskPayload, run *model.MallGeocodeRun, candidates []model.MallGeocodeCandidate, autoIndex int, outcome mallGeocodeOutcome) error {
	updates := map[string]interface{}{"updated_by": uint(0)}
	if outcome.ProviderErr != nil {
		var providerError *geocoder.ProviderError
		if errors.As(outcome.ProviderErr, &providerError) && providerError.Retryable {
			return nil
		}
		updates["geocode_status"] = "failed"
	} else if len(candidates) == 0 {
		updates["geocode_status"] = "failed"
	} else if autoIndex < 0 {
		updates["geocode_status"] = "review"
		updates["status"] = "geocode_review"
	} else {
		candidate := candidates[autoIndex]
		now := outcome.FinishedAt.UTC()
		updates["longitude"] = candidate.Longitude
		updates["latitude"] = candidate.Latitude
		updates["coordinate_system"] = candidate.CoordinateSystem
		updates["weather_longitude"] = candidate.Longitude
		updates["weather_latitude"] = candidate.Latitude
		updates["weather_coordinate_system"] = candidate.CoordinateSystem
		updates["address_standardized"] = candidate.FormattedAddress
		updates["adcode"] = candidate.Adcode
		updates["citycode"] = candidate.Citycode
		updates["geocode_level"] = candidate.Level
		updates["geocode_confidence"] = candidate.ConfidenceScore
		updates["geocode_status"] = "confirmed"
		updates["geocoded_at"] = now
		updates["weather_enabled"] = true
		updates["status"] = "active"
		run.SelectedCandidateID = &candidate.ID
		if err := tx.Model(&model.MallGeocodeRun{}).Where("id = ?", run.ID).
			Update("selected_candidate_id", candidate.ID).Error; err != nil {
			return fmt.Errorf("mall geocode: mark auto-selected candidate: %w", err)
		}
	}
	if err := data_dao.NewMallDAO(tx).UpdateWithVersion(ctx, payload.MallID, payload.MallVersion, updates); err != nil {
		return err
	}
	if autoIndex >= 0 && outcome.ProviderErr == nil {
		return createInitialWeatherOutboxes(ctx, tx, payload.MallID, payload.MallVersion+1, outcome.FinishedAt)
	}
	return nil
}

func createInitialWeatherOutboxes(ctx context.Context, tx *gorm.DB, mallID uint, version uint64, now time.Time) error {
	rows, err := newInitialWeatherOutboxes(mallID, version, now)
	if err != nil {
		return err
	}
	dao := data_dao.NewAsyncJobOutboxDAO(tx)
	for _, row := range rows {
		if err := dao.Create(ctx, row); err != nil {
			return err
		}
	}
	return nil
}

func newInitialWeatherOutboxes(mallID uint, version uint64, now time.Time) ([]*model.AsyncJobOutbox, error) {
	if mallID == 0 || version == 0 {
		return nil, fmt.Errorf("mall geocode: invalid initial weather identity")
	}
	fullWindow := fmt.Sprintf("full:%d:init:v%d", mallID, version)
	fullPayloadJSON, err := json.Marshal(job.MallTaskPayload{MallID: mallID, TaskWindow: fullWindow})
	if err != nil {
		return nil, fmt.Errorf("mall geocode: encode initial full weather payload: %w", err)
	}
	rows := []*model.AsyncJobOutbox{
		{TaskKey: "mall:weather:" + fullWindow, TaskType: job.TypeMallWeatherFull, PayloadJSON: model.JSONText(fullPayloadJSON), QueueName: job.MallWeatherQueueName, AvailableAt: now.UTC()},
	}
	return rows, nil
}

func setSafeGeocodeRunError(run *model.MallGeocodeRun, err error) {
	run.ErrorClass = "provider"
	run.ErrorCode = "GEOCODE_FAILED"
	run.ErrorMessageSafe = "geocoding provider request failed"
	var providerError *geocoder.ProviderError
	if errors.As(err, &providerError) {
		run.ErrorClass = string(providerError.Class)
		if providerError.Code != "" {
			run.ErrorCode = providerError.Code
		}
	}
}
