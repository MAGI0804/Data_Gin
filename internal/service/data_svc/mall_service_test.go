package data_svc

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/model"

	mysqlDriver "github.com/go-sql-driver/mysql"
)

func TestNormalizeMallCreateRequestCanonicalizesCallerInputWithoutConfigDefaults(t *testing.T) {
	request := validMallCreateRequest()
	request.MallCode = " sh-hl-001 "
	request.NameCN = " 示例商场 "
	request.Tags = []string{"地铁直达", "购物中心", "地铁直达"}
	request.BusinessHours = map[string][]requestbody.MallBusinessHour{" Monday ": {{Open: "10:00", Close: "22:00"}}}

	normalized, err := normalizeMallCreateRequest(request)
	if err != nil {
		t.Fatalf("normalizeMallCreateRequest() error = %v", err)
	}
	if normalized.MallCode != "SH-HL-001" || normalized.NameCN != "示例商场" {
		t.Fatalf("normalized request = %+v", normalized)
	}
	if strings.Join(normalized.Tags, ",") != "地铁直达,购物中心" {
		t.Fatalf("tags = %v", normalized.Tags)
	}
	if normalized.Weather.DetailProfile != nil || normalized.Weather.CoverageRadiusM != nil {
		t.Fatal("normalization injected runtime defaults before idempotency hashing")
	}
	if _, ok := normalized.BusinessHours["monday"]; !ok {
		t.Fatalf("business hour keys were not canonicalized: %#v", normalized.BusinessHours)
	}
}

func TestApplyMallWeatherDefaultsAndBuildModel(t *testing.T) {
	request, err := normalizeMallCreateRequest(validMallCreateRequest())
	if err != nil {
		t.Fatalf("normalizeMallCreateRequest() error = %v", err)
	}
	if err := applyMallWeatherDefaults(&request, "full", 1000); err != nil {
		t.Fatalf("applyMallWeatherDefaults() error = %v", err)
	}
	now := time.Date(2026, 7, 17, 10, 0, 0, 0, time.UTC)
	mall, err := mallFromCreateRequest(request, 9, now)
	if err != nil {
		t.Fatalf("mallFromCreateRequest() error = %v", err)
	}
	if mall.Status != "draft" || mall.GeocodeStatus != "pending" || mall.WeatherEnabled {
		t.Fatalf("mall lifecycle = status=%s geocode=%s weather=%v", mall.Status, mall.GeocodeStatus, mall.WeatherEnabled)
	}
	if mall.WeatherProvider != "caiyun" || mall.SamplingMode != "center" || mall.Version != 1 {
		t.Fatalf("mall weather defaults = %+v", mall)
	}
	if !mall.CreatedAt.Equal(now) || !mall.UpdatedAt.Equal(now) || mall.CreatedBy != 9 || mall.UpdatedBy != 9 {
		t.Fatalf("mall audit fields = %+v", mall)
	}
}

func TestNormalizeMallCreateRequestRejectsInvalidBoundaries(t *testing.T) {
	negativeParking := -1
	tests := []struct {
		name   string
		mutate func(*requestbody.MallCreateRequest)
	}{
		{"invalid code", func(request *requestbody.MallCreateRequest) { request.MallCode = "bad code" }},
		{"missing name", func(request *requestbody.MallCreateRequest) { request.NameCN = "" }},
		{"negative parking", func(request *requestbody.MallCreateRequest) { request.ParkingSpaces = &negativeParking }},
		{"invalid business hour", func(request *requestbody.MallCreateRequest) {
			request.BusinessHours = map[string][]requestbody.MallBusinessHour{"monday": {{Open: "25:00", Close: "22:00"}}}
		}},
		{"unknown day", func(request *requestbody.MallCreateRequest) {
			request.BusinessHours = map[string][]requestbody.MallBusinessHour{"holiday": {{Open: "10:00", Close: "22:00"}}}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := validMallCreateRequest()
			tt.mutate(&request)
			if _, err := normalizeMallCreateRequest(request); !errors.Is(err, ErrMallInvalidInput) {
				t.Fatalf("normalizeMallCreateRequest() error = %v", err)
			}
		})
	}
}

func TestBuildMallPatchInvalidatesCoordinatesAndQueuesNewVersion(t *testing.T) {
	longitude, latitude := 121.4, 31.2
	current := &model.Mall{
		BaseModel: model.BaseModel{ID: 7}, NameCN: "旧商场", Province: "上海市", City: "上海市",
		District: "静安区", AddressRaw: "旧地址", Longitude: &longitude, Latitude: &latitude,
		CoordinateSystem: "GCJ02", GeocodeStatus: "confirmed", WeatherEnabled: true, Status: "active", Version: 4,
	}
	address := "新地址"
	updates, requiresGeocode, err := buildMallPatch(current, requestbody.MallPatchRequest{
		ExpectedMallVersion: 4,
		Address:             &address,
	}, 9)
	if err != nil {
		t.Fatalf("buildMallPatch() error = %v", err)
	}
	if !requiresGeocode || updates["longitude"] != nil || updates["geocode_status"] != "pending" || updates["weather_enabled"] != false || updates["status"] != "draft" {
		t.Fatalf("updates = %#v", updates)
	}
	candidate := *current
	applyMallUpdatesForOutbox(&candidate, updates)
	candidate.Version = 5
	outbox, err := newMallGeocodeOutbox(&candidate, time.Now())
	if err != nil {
		t.Fatalf("newMallGeocodeOutbox() error = %v", err)
	}
	if !strings.Contains(outbox.TaskKey, "mall:geocode:7:v5:") || strings.Contains(string(outbox.PayloadJSON), "新地址") {
		t.Fatalf("outbox = %+v", outbox)
	}
}

func TestBuildMallPatchCannotEnableWeatherBeforeConfirmation(t *testing.T) {
	enabled := true
	_, _, err := buildMallPatch(&model.Mall{GeocodeStatus: "pending"}, requestbody.MallPatchRequest{
		ExpectedMallVersion: 1,
		Weather:             &requestbody.MallWeatherSettingsRequest{Enabled: &enabled},
	}, 1)
	if !errors.Is(err, ErrMallInvalidInput) {
		t.Fatalf("buildMallPatch() error = %v", err)
	}
}

func TestMallDTORejectsCorruptStoredJSON(t *testing.T) {
	if _, err := mallDTO(&model.Mall{BusinessHoursJSON: `{`}); err == nil {
		t.Fatal("mallDTO() accepted invalid business hours JSON")
	}
	if _, err := mallDTO(&model.Mall{TagsJSON: `[`}); err == nil {
		t.Fatal("mallDTO() accepted invalid tags JSON")
	}
}

func TestMallServiceAuthorizationFailsClosed(t *testing.T) {
	service := &MallService{
		permissions: fakeMallPermissionChecker{allowed: false},
		now:         func() time.Time { return time.Now() },
	}
	if _, err := service.Get(context.Background(), 10, 1); !errors.Is(err, ErrMallForbidden) {
		t.Fatalf("Get() error = %v, want ErrMallForbidden", err)
	}

	service.permissions = fakeMallPermissionChecker{err: errors.New("database unavailable")}
	if _, err := service.Get(context.Background(), 10, 1); err == nil || errors.Is(err, ErrMallForbidden) {
		t.Fatalf("Get() error = %v, want internal authorization error", err)
	}
}

func TestMallIdempotencyAndDuplicateHelpers(t *testing.T) {
	if !validIdempotencyKey("84c2e4a0-1234-4567-8901-123456789012") {
		t.Fatal("validIdempotencyKey() rejected UUID-shaped key")
	}
	for _, value := range []string{"short", "bad key with spaces", strings.Repeat("a", 256)} {
		if validIdempotencyKey(value) {
			t.Fatalf("validIdempotencyKey(%q) = true", value)
		}
	}
	if !isDuplicateKeyError(&mysqlDriver.MySQLError{Number: 1062}) || isDuplicateKeyError(errors.New("duplicate")) {
		t.Fatal("isDuplicateKeyError() misclassified error")
	}
}

type fakeMallPermissionChecker struct {
	allowed bool
	err     error
}

func (checker fakeMallPermissionChecker) HasPermission(context.Context, uint, string, time.Time) (bool, error) {
	return checker.allowed, checker.err
}

func validMallCreateRequest() requestbody.MallCreateRequest {
	return requestbody.MallCreateRequest{
		MallCode: "SH-HL-001",
		NameCN:   "示例商场",
		Province: "上海市",
		City:     "上海市",
		District: "静安区",
		Address:  "南京西路1266号",
		BusinessHours: map[string][]requestbody.MallBusinessHour{
			"monday": {{Open: "10:00", Close: "22:00"}},
		},
		Tags: []string{"购物中心"},
	}
}
