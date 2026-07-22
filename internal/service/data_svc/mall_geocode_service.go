package data_svc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/model"

	"gorm.io/gorm"
)

type MallGeocodeTriggerResult struct {
	JobID         uint   `json:"jobId"`
	MallID        uint   `json:"mallId"`
	MallVersion   uint64 `json:"mallVersion"`
	GeocodeStatus string `json:"geocodeStatus"`
}

type MallGeocodeCandidateDTO struct {
	ID               uint     `json:"id"`
	CandidateNo      int      `json:"candidateNo"`
	FormattedAddress string   `json:"formattedAddress"`
	Province         string   `json:"province,omitempty"`
	City             string   `json:"city,omitempty"`
	District         string   `json:"district,omitempty"`
	Adcode           string   `json:"adcode,omitempty"`
	Citycode         string   `json:"citycode,omitempty"`
	Longitude        float64  `json:"longitude"`
	Latitude         float64  `json:"latitude"`
	CoordinateSystem string   `json:"coordinateSystem"`
	Level            string   `json:"level,omitempty"`
	ConfidenceScore  float64  `json:"confidenceScore"`
	ScoreReasons     []string `json:"scoreReasons"`
	Selected         bool     `json:"selected"`
}

type MallGeocodeCandidatesResult struct {
	MallID      uint                      `json:"mallId"`
	MallVersion uint64                    `json:"mallVersion"`
	RunID       uint                      `json:"runId,omitempty"`
	RunStatus   string                    `json:"runStatus,omitempty"`
	Items       []MallGeocodeCandidateDTO `json:"items"`
}

type mallGeocodeConfirmation struct {
	longitude        float64
	latitude         float64
	coordinateSystem string
	reason           string
	source           string
	run              *model.MallGeocodeRun
	candidate        *model.MallGeocodeCandidate
}

func (service *MallService) TriggerGeocode(ctx context.Context, actorUserID, mallID uint, expectedVersion uint64) (*MallGeocodeTriggerResult, error) {
	if err := service.authorize(ctx, actorUserID, PermissionMallWrite); err != nil {
		return nil, err
	}
	if mallID == 0 || expectedVersion == 0 {
		return nil, fmt.Errorf("%w: mall id and expectedMallVersion are required", ErrMallInvalidInput)
	}

	var result *MallGeocodeTriggerResult
	err := service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		mallDAO := data_dao.NewMallDAO(tx)
		mall, err := mallDAO.FindByIDForUpdate(ctx, mallID)
		if err != nil {
			return err
		}
		if mall.Version != expectedVersion {
			return data_dao.ErrMallVersionConflict
		}
		updates := map[string]interface{}{
			"geocode_status": "pending",
			"updated_by":     actorUserID,
		}
		if mall.Longitude == nil || mall.Latitude == nil {
			updates["status"] = "draft"
		}
		if err := mallDAO.UpdateWithVersion(ctx, mallID, expectedVersion, updates); err != nil {
			return err
		}

		queuedMall := *mall
		queuedMall.Version = expectedVersion + 1
		queuedMall.GeocodeStatus = "pending"
		outbox, err := newMallGeocodeOutbox(&queuedMall, service.now().UTC())
		if err != nil {
			return err
		}
		if err := data_dao.NewAsyncJobOutboxDAO(tx).Create(ctx, outbox); err != nil {
			return fmt.Errorf("mall service: create triggered geocode outbox: %w", err)
		}
		result = &MallGeocodeTriggerResult{
			JobID: outbox.ID, MallID: mallID, MallVersion: queuedMall.Version, GeocodeStatus: "PENDING",
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (service *MallService) ListGeocodeCandidates(ctx context.Context, actorUserID, mallID uint) (*MallGeocodeCandidatesResult, error) {
	if err := service.authorize(ctx, actorUserID, PermissionMallRead); err != nil {
		return nil, err
	}
	if mallID == 0 {
		return nil, fmt.Errorf("%w: mall id is required", ErrMallInvalidInput)
	}
	mall, err := data_dao.NewMallDAO(service.db).FindByID(ctx, mallID)
	if err != nil {
		return nil, err
	}
	result := &MallGeocodeCandidatesResult{
		MallID: mallID, MallVersion: mall.Version, Items: make([]MallGeocodeCandidateDTO, 0),
	}
	geocodeDAO := data_dao.NewMallGeocodeDAO(service.db)
	run, err := geocodeDAO.FindLatestRun(ctx, mallID)
	if errors.Is(err, data_dao.ErrMallGeocodeRunNotFound) {
		return result, nil
	}
	if err != nil {
		return nil, err
	}
	result.RunID = run.ID
	result.RunStatus = strings.ToUpper(run.Status)
	if run.AddressHash != mallAddressHash(mall) {
		result.RunStatus = "STALE"
		return result, nil
	}
	rows, err := geocodeDAO.ListCandidates(ctx, mallID, run.ID)
	if err != nil {
		return nil, err
	}
	for index := range rows {
		item, err := mallGeocodeCandidateDTO(&rows[index])
		if err != nil {
			return nil, err
		}
		result.Items = append(result.Items, item)
	}
	return result, nil
}

func (service *MallService) ConfirmGeocode(ctx context.Context, actorUserID, mallID uint, request requestbody.MallGeocodeConfirmRequest) (*MallDTO, error) {
	if err := service.authorize(ctx, actorUserID, PermissionMallGeocodeConfirm); err != nil {
		return nil, err
	}
	if mallID == 0 || request.ExpectedMallVersion == 0 {
		return nil, fmt.Errorf("%w: mall id and expectedMallVersion are required", ErrMallInvalidInput)
	}
	if err := validateMallGeocodeConfirmationRequest(request); err != nil {
		return nil, err
	}

	var updated *model.Mall
	err := service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		mallDAO := data_dao.NewMallDAO(tx)
		mall, err := mallDAO.FindByIDForUpdate(ctx, mallID)
		if err != nil {
			return err
		}
		if mall.Version != request.ExpectedMallVersion {
			return data_dao.ErrMallVersionConflict
		}

		geocodeDAO := data_dao.NewMallGeocodeDAO(tx)
		confirmation, err := resolveMallGeocodeConfirmation(ctx, geocodeDAO, mall, request)
		if err != nil {
			return err
		}
		now := service.now().UTC()
		updates := mallGeocodeConfirmationUpdates(confirmation, request.WeatherEnabled, actorUserID, now)
		if err := mallDAO.UpdateWithVersion(ctx, mallID, request.ExpectedMallVersion, updates); err != nil {
			return err
		}
		if confirmation.candidate != nil {
			if err := geocodeDAO.MarkSelectedForMall(ctx, mallID, confirmation.run.ID, confirmation.candidate.ID); err != nil {
				return err
			}
		}
		audit := newMallCoordinateAudit(mall, confirmation, actorUserID, now)
		if err := geocodeDAO.CreateCoordinateAudit(ctx, audit); err != nil {
			return err
		}
		if request.WeatherEnabled {
			if err := createInitialWeatherOutboxes(ctx, tx, mallID, request.ExpectedMallVersion+1, now); err != nil {
				return fmt.Errorf("mall service: create initial weather outboxes: %w", err)
			}
		}
		updated, err = mallDAO.FindByID(ctx, mallID)
		return err
	})
	if err != nil {
		return nil, err
	}
	dto, err := mallDTO(updated)
	if err != nil {
		return nil, err
	}
	return &dto, nil
}

func validateMallGeocodeConfirmationRequest(request requestbody.MallGeocodeConfirmRequest) error {
	if (request.CandidateID == nil) == (request.ManualCoordinate == nil) {
		return fmt.Errorf("%w: exactly one coordinate source is required", ErrMallInvalidInput)
	}
	if request.CandidateID != nil {
		if *request.CandidateID == 0 {
			return fmt.Errorf("%w: candidateId is required", ErrMallInvalidInput)
		}
		return nil
	}
	manual := request.ManualCoordinate
	manual.CoordinateSystem = strings.ToUpper(strings.TrimSpace(manual.CoordinateSystem))
	manual.Reason = strings.TrimSpace(manual.Reason)
	if !validCoordinate(manual.Longitude, manual.Latitude) {
		return fmt.Errorf("%w: invalid manual coordinate", ErrMallInvalidInput)
	}
	if manual.CoordinateSystem != "GCJ02" {
		return fmt.Errorf("%w: coordinateSystem must be GCJ02", ErrMallInvalidInput)
	}
	if !validText(manual.Reason, 1, 500) {
		return fmt.Errorf("%w: manual coordinate reason is required", ErrMallInvalidInput)
	}
	return nil
}

func resolveMallGeocodeConfirmation(ctx context.Context, dao *data_dao.MallGeocodeDAO, mall *model.Mall, request requestbody.MallGeocodeConfirmRequest) (*mallGeocodeConfirmation, error) {
	if request.ManualCoordinate != nil {
		manual := request.ManualCoordinate
		return &mallGeocodeConfirmation{
			longitude: manual.Longitude, latitude: manual.Latitude,
			coordinateSystem: strings.ToUpper(strings.TrimSpace(manual.CoordinateSystem)),
			reason:           strings.TrimSpace(manual.Reason), source: "manual",
		}, nil
	}
	run, err := dao.FindLatestRunForUpdate(ctx, mall.ID)
	if err != nil {
		return nil, err
	}
	candidate, err := dao.FindCandidateForUpdate(ctx, mall.ID, *request.CandidateID)
	if err != nil {
		return nil, err
	}
	if candidate.RunID != run.ID || run.AddressHash != mallAddressHash(mall) {
		return nil, fmt.Errorf("%w: geocode candidate is stale", ErrMallConflict)
	}
	if !validCoordinate(candidate.Longitude, candidate.Latitude) || strings.ToUpper(candidate.CoordinateSystem) != "GCJ02" {
		return nil, fmt.Errorf("%w: candidate coordinate is invalid", ErrMallInvalidInput)
	}
	return &mallGeocodeConfirmation{
		longitude: candidate.Longitude, latitude: candidate.Latitude,
		coordinateSystem: "GCJ02", reason: "selected geocode candidate", source: "candidate",
		run: run, candidate: candidate,
	}, nil
}

func mallGeocodeConfirmationUpdates(confirmation *mallGeocodeConfirmation, weatherEnabled bool, actorUserID uint, now time.Time) map[string]interface{} {
	updates := map[string]interface{}{
		"longitude":                 confirmation.longitude,
		"latitude":                  confirmation.latitude,
		"coordinate_system":         confirmation.coordinateSystem,
		"weather_longitude":         confirmation.longitude,
		"weather_latitude":          confirmation.latitude,
		"weather_coordinate_system": confirmation.coordinateSystem,
		"geocode_status":            "confirmed",
		"geocoded_at":               now,
		"geocode_confirmed_by":      actorUserID,
		"weather_enabled":           weatherEnabled,
		"status":                    "active",
		"updated_by":                actorUserID,
	}
	if confirmation.candidate == nil {
		updates["geocode_level"] = "manual"
		updates["geocode_confidence"] = nil
		return updates
	}
	candidate := confirmation.candidate
	updates["address_standardized"] = candidate.FormattedAddress
	updates["adcode"] = candidate.Adcode
	updates["citycode"] = candidate.Citycode
	updates["geocode_level"] = candidate.Level
	updates["geocode_confidence"] = candidate.ConfidenceScore
	return updates
}

func newMallCoordinateAudit(mall *model.Mall, confirmation *mallGeocodeConfirmation, actorUserID uint, now time.Time) *model.MallCoordinateAudit {
	afterLongitude := confirmation.longitude
	afterLatitude := confirmation.latitude
	audit := &model.MallCoordinateAudit{
		MallID: mall.ID, Source: confirmation.source,
		BeforeLongitude: mall.Longitude, BeforeLatitude: mall.Latitude,
		BeforeCoordinateSystem: mall.CoordinateSystem,
		AfterLongitude:         &afterLongitude, AfterLatitude: &afterLatitude,
		AfterCoordinateSystem: confirmation.coordinateSystem,
		Reason:                confirmation.reason, ConfirmedBy: actorUserID,
		MallVersionBefore: mall.Version, MallVersionAfter: mall.Version + 1,
		ConfirmedAt: now.UTC(),
	}
	if confirmation.run != nil {
		runID := confirmation.run.ID
		audit.RunID = &runID
	}
	if confirmation.candidate != nil {
		candidateID := confirmation.candidate.ID
		audit.CandidateID = &candidateID
	}
	return audit
}

func mallGeocodeCandidateDTO(candidate *model.MallGeocodeCandidate) (MallGeocodeCandidateDTO, error) {
	reasons := make([]string, 0)
	if strings.TrimSpace(string(candidate.ScoreReasonsJSON)) != "" {
		if err := json.Unmarshal([]byte(candidate.ScoreReasonsJSON), &reasons); err != nil {
			return MallGeocodeCandidateDTO{}, fmt.Errorf("mall service: decode candidate score reasons: %w", err)
		}
		if reasons == nil {
			reasons = make([]string, 0)
		}
	}
	return MallGeocodeCandidateDTO{
		ID: candidate.ID, CandidateNo: candidate.CandidateNo,
		FormattedAddress: candidate.FormattedAddress, Province: candidate.Province,
		City: candidate.City, District: candidate.District, Adcode: candidate.Adcode,
		Citycode: candidate.Citycode, Longitude: candidate.Longitude, Latitude: candidate.Latitude,
		CoordinateSystem: candidate.CoordinateSystem, Level: candidate.Level,
		ConfidenceScore: candidate.ConfidenceScore, ScoreReasons: reasons, Selected: candidate.IsSelected,
	}, nil
}

func validCoordinate(longitude, latitude float64) bool {
	return !math.IsNaN(longitude) && !math.IsInf(longitude, 0) && longitude >= -180 && longitude <= 180 &&
		!math.IsNaN(latitude) && !math.IsInf(latitude, 0) && latitude >= -90 && latitude <= 90
}
