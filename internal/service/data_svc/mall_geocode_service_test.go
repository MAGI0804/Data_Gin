package data_svc

import (
	"errors"
	"math"
	"testing"
	"time"

	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/model"
)

func TestValidateMallGeocodeConfirmationRequest(t *testing.T) {
	candidateID := uint(7)
	tests := []struct {
		name    string
		request requestbody.MallGeocodeConfirmRequest
		wantErr bool
	}{
		{name: "candidate", request: requestbody.MallGeocodeConfirmRequest{CandidateID: &candidateID}},
		{name: "manual", request: requestbody.MallGeocodeConfirmRequest{ManualCoordinate: &requestbody.MallManualCoordinateRequest{
			Longitude: 121.4551234, Latitude: 31.2285678, CoordinateSystem: " gcj02 ", Reason: " correction ",
		}}},
		{name: "missing source", request: requestbody.MallGeocodeConfirmRequest{}, wantErr: true},
		{name: "both sources", request: requestbody.MallGeocodeConfirmRequest{CandidateID: &candidateID, ManualCoordinate: &requestbody.MallManualCoordinateRequest{}}, wantErr: true},
		{name: "zero candidate", request: requestbody.MallGeocodeConfirmRequest{CandidateID: new(uint)}, wantErr: true},
		{name: "wrong coordinate system", request: requestbody.MallGeocodeConfirmRequest{ManualCoordinate: &requestbody.MallManualCoordinateRequest{
			Longitude: 121.4, Latitude: 31.2, CoordinateSystem: "WGS84", Reason: "correction",
		}}, wantErr: true},
		{name: "missing reason", request: requestbody.MallGeocodeConfirmRequest{ManualCoordinate: &requestbody.MallManualCoordinateRequest{
			Longitude: 121.4, Latitude: 31.2, CoordinateSystem: "GCJ02",
		}}, wantErr: true},
		{name: "invalid coordinate", request: requestbody.MallGeocodeConfirmRequest{ManualCoordinate: &requestbody.MallManualCoordinateRequest{
			Longitude: math.NaN(), Latitude: 31.2, CoordinateSystem: "GCJ02", Reason: "correction",
		}}, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateMallGeocodeConfirmationRequest(test.request)
			if test.wantErr != errors.Is(err, ErrMallInvalidInput) {
				t.Fatalf("error=%v wantInvalid=%t", err, test.wantErr)
			}
		})
	}
}

func TestMallGeocodeConfirmationBuildsAuditAndUpdates(t *testing.T) {
	beforeLongitude := 121.1
	beforeLatitude := 31.1
	candidate := &model.MallGeocodeCandidate{
		BaseModel: model.BaseModel{ID: 987}, RunID: 44, FormattedAddress: "标准地址",
		Adcode: "310106", Citycode: "021", Longitude: 121.4551234, Latitude: 31.2285678,
		CoordinateSystem: "GCJ02", Level: "兴趣点", ConfidenceScore: 92.5,
	}
	run := &model.MallGeocodeRun{BaseModel: model.BaseModel{ID: 44}}
	confirmation := &mallGeocodeConfirmation{
		longitude: candidate.Longitude, latitude: candidate.Latitude, coordinateSystem: "GCJ02",
		reason: "selected geocode candidate", source: "candidate", run: run, candidate: candidate,
	}
	now := time.Date(2026, 7, 22, 8, 0, 0, 0, time.UTC)
	mall := &model.Mall{
		BaseModel: model.BaseModel{ID: 9}, Longitude: &beforeLongitude, Latitude: &beforeLatitude,
		CoordinateSystem: "GCJ02", Version: 5,
	}

	updates := mallGeocodeConfirmationUpdates(confirmation, true, 17, now)
	if updates["geocode_confirmed_by"] != uint(17) || updates["weather_enabled"] != true || updates["status"] != "active" || updates["address_standardized"] != "标准地址" {
		t.Fatalf("updates=%+v", updates)
	}
	audit := newMallCoordinateAudit(mall, confirmation, 17, now)
	if audit.ConfirmedBy != 17 || audit.MallVersionBefore != 5 || audit.MallVersionAfter != 6 || audit.RunID == nil || *audit.RunID != 44 || audit.CandidateID == nil || *audit.CandidateID != 987 {
		t.Fatalf("audit=%+v", audit)
	}
	if audit.BeforeLongitude == nil || *audit.BeforeLongitude != beforeLongitude || audit.AfterLongitude == nil || *audit.AfterLongitude != candidate.Longitude {
		t.Fatalf("audit coordinates=%+v", audit)
	}
}

func TestManualGeocodeConfirmationDoesNotInventProviderProvenance(t *testing.T) {
	confirmation := &mallGeocodeConfirmation{
		longitude: 121.4, latitude: 31.2, coordinateSystem: "GCJ02", reason: "main entrance", source: "manual",
	}
	updates := mallGeocodeConfirmationUpdates(confirmation, false, 17, time.Now())
	if updates["geocode_level"] != "manual" || updates["geocode_confidence"] != nil {
		t.Fatalf("updates=%+v", updates)
	}
	audit := newMallCoordinateAudit(&model.Mall{BaseModel: model.BaseModel{ID: 9}, Version: 1}, confirmation, 17, time.Now())
	if audit.RunID != nil || audit.CandidateID != nil || audit.Source != "manual" || audit.Reason != "main entrance" {
		t.Fatalf("audit=%+v", audit)
	}
}

func TestMallGeocodeCandidateDTORejectsCorruptReasons(t *testing.T) {
	if _, err := mallGeocodeCandidateDTO(&model.MallGeocodeCandidate{ScoreReasonsJSON: model.JSONText(`{`)}); err == nil {
		t.Fatal("mallGeocodeCandidateDTO() error=nil")
	}
	item, err := mallGeocodeCandidateDTO(&model.MallGeocodeCandidate{ScoreReasonsJSON: model.JSONText(`["city_match"]`)})
	if err != nil || len(item.ScoreReasons) != 1 || item.ScoreReasons[0] != "city_match" {
		t.Fatalf("item=%+v error=%v", item, err)
	}
}
