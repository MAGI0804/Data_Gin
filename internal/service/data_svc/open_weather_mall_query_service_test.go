package data_svc

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/model"
)

type fakeOpenWeatherMallReader struct {
	afterID uint
	limit   int
	malls   []model.Mall
	calls   int
	counts  int
}

func (reader *fakeOpenWeatherMallReader) CountOpenWeatherMalls(context.Context) (int64, error) {
	reader.counts++
	return int64(len(reader.malls)), nil
}

func (reader *fakeOpenWeatherMallReader) ListOpenWeatherMallsAfterID(
	_ context.Context,
	afterID uint,
	limit int,
) ([]model.Mall, error) {
	reader.calls++
	reader.afterID, reader.limit = afterID, limit
	return reader.malls, nil
}

type fakeOpenWeatherMallPermissionReader struct {
	allowed    bool
	permission string
}

func (reader *fakeOpenWeatherMallPermissionReader) HasPermission(
	_ context.Context,
	_ uint,
	permission string,
	_ time.Time,
) (bool, error) {
	reader.permission = permission
	return reader.allowed, nil
}

func TestOpenWeatherMallQueryServiceReturnsPublicCursorPage(t *testing.T) {
	weatherLongitude, weatherLatitude := 121.502, 31.240
	malls := &fakeOpenWeatherMallReader{malls: []model.Mall{
		{
			BaseModel: model.BaseModel{ID: 7}, MallCode: "M001", NameCN: "上海某商场",
			NameEN: "Shanghai Mall", Country: "中国", Province: "上海市", City: "上海市", District: "浦东新区",
			Township: "陆家嘴街道", Street: "世纪大道", StreetNumber: "1号", AddressRaw: "浦东陆家嘴附近",
			AddressStandardized: "上海市浦东新区世纪大道1号", WeatherLongitude: &weatherLongitude,
			WeatherLatitude: &weatherLatitude, WeatherCoordinateSystem: "gcj02",
			Timezone: "Asia/Shanghai", WeatherEnabled: true, ContactPhone: "secret",
		},
		{
			BaseModel: model.BaseModel{ID: 8}, MallCode: "M002", WeatherEnabled: true,
			WeatherLongitude: &weatherLongitude, WeatherLatitude: &weatherLatitude,
		},
	}}
	permissions := &fakeOpenWeatherMallPermissionReader{allowed: true}
	service := newOpenWeatherMallQueryService(malls, permissions, time.Now)
	result, err := service.Query(t.Context(), 17, requestbody.OpenWeatherMallQueryRequest{PageSize: 1})
	if err != nil {
		t.Fatalf("Query() error=%v", err)
	}
	if permissions.permission != model.PermissionWeatherRead || malls.calls != 1 || malls.counts != 1 || malls.afterID != 0 || malls.limit != 2 {
		t.Fatalf("permission=%q calls=%d afterID=%d limit=%d", permissions.permission, malls.calls, malls.afterID, malls.limit)
	}
	if len(result.Items) != 1 || result.Items[0].MallID != 7 || result.Items[0].TimeZone != "Asia/Shanghai" ||
		!result.Pagination.HasMore || result.Pagination.NextCursor == "" {
		t.Fatalf("result=%+v", result)
	}
	item := result.Items[0]
	if item.Location != "上海市 浦东新区 陆家嘴街道" || item.Address != "上海市浦东新区世纪大道1号" ||
		item.Longitude == nil || *item.Longitude != weatherLongitude || item.Latitude == nil ||
		*item.Latitude != weatherLatitude || item.CoordinateSystem != "GCJ02" {
		t.Fatalf("mall item=%+v", item)
	}
	if result.Pagination.Page != 1 || result.Pagination.PageSize != 1 || result.Pagination.TotalItems != 2 || result.Pagination.TotalPages != 2 {
		t.Fatalf("pagination totals=%+v", result.Pagination)
	}
	payload, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	for _, forbidden := range []string{
		"secret", "contactPhone", "addressRaw", "addressStandardized", "weatherLongitude",
		"weatherLatitude", "weatherCoordinateSystem", "weatherEnabled", "geocodeConfidence",
	} {
		if strings.Contains(string(payload), forbidden) {
			t.Fatalf("response contains redundant or internal field %q: %s", forbidden, payload)
		}
	}
	var decodedResult struct {
		Items []map[string]json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(payload, &decodedResult); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(decodedResult.Items) != 1 || len(decodedResult.Items[0]) != 10 {
		t.Fatalf("public mall fields=%v", decodedResult.Items)
	}
	for _, field := range []string{
		"mallId", "mallCode", "nameCn", "nameEn", "location", "address",
		"longitude", "latitude", "coordinateSystem", "timeZone",
	} {
		if _, exists := decodedResult.Items[0][field]; !exists {
			t.Fatalf("public mall response missing %q: %s", field, payload)
		}
	}
	cursor, err := decodeOpenWeatherMallCursor(result.Pagination.NextCursor)
	if err != nil || cursor.ID != 7 || cursor.Version != 1 || cursor.Page != 2 {
		t.Fatalf("cursor=%+v error=%v", cursor, err)
	}
}

func TestOpenWeatherMallAddressFallsBackToRawAddress(t *testing.T) {
	mall := &model.Mall{AddressRaw: "  浦东陆家嘴附近  "}
	if address := openWeatherMallAddress(mall); address != "浦东陆家嘴附近" {
		t.Fatalf("address=%q", address)
	}
}

func TestOpenWeatherMallQueryServiceContinuesFromCursor(t *testing.T) {
	cursor, err := encodeOpenWeatherMallCursor(7, 2)
	if err != nil {
		t.Fatalf("encode cursor: %v", err)
	}
	malls := &fakeOpenWeatherMallReader{malls: []model.Mall{}}
	service := newOpenWeatherMallQueryService(
		malls,
		&fakeOpenWeatherMallPermissionReader{allowed: true},
		time.Now,
	)
	result, err := service.Query(t.Context(), 17, requestbody.OpenWeatherMallQueryRequest{Cursor: cursor})
	if err != nil {
		t.Fatalf("Query() error=%v", err)
	}
	if malls.afterID != 7 || malls.limit != openWeatherMallDefaultPageSize+1 || result.Items == nil {
		t.Fatalf("afterID=%d limit=%d result=%+v", malls.afterID, malls.limit, result)
	}
}

func TestOpenWeatherMallQueryServiceRejectsInvalidQueryBeforeDAO(t *testing.T) {
	malls := &fakeOpenWeatherMallReader{}
	service := newOpenWeatherMallQueryService(
		malls,
		&fakeOpenWeatherMallPermissionReader{allowed: true},
		time.Now,
	)
	for _, request := range []requestbody.OpenWeatherMallQueryRequest{
		{PageSize: -1},
		{PageSize: openWeatherMallMaxPageSize + 1},
		{Cursor: "%%%"},
	} {
		if _, err := service.Query(t.Context(), 17, request); !errors.Is(err, ErrOpenWeatherMallInvalidQuery) {
			t.Fatalf("Query(%+v) error=%v", request, err)
		}
	}
	if malls.calls != 0 {
		t.Fatalf("DAO calls=%d", malls.calls)
	}
}

func TestOpenWeatherMallQueryServiceRequiresWeatherReadPermission(t *testing.T) {
	malls := &fakeOpenWeatherMallReader{}
	service := newOpenWeatherMallQueryService(
		malls,
		&fakeOpenWeatherMallPermissionReader{allowed: false},
		time.Now,
	)
	_, err := service.Query(t.Context(), 17, requestbody.OpenWeatherMallQueryRequest{})
	if !errors.Is(err, ErrOpenWeatherMallForbidden) || malls.calls != 0 {
		t.Fatalf("error=%v DAO calls=%d", err, malls.calls)
	}
}
