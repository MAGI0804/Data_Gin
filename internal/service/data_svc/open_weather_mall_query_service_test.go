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
	malls := &fakeOpenWeatherMallReader{malls: []model.Mall{
		{
			BaseModel: model.BaseModel{ID: 7}, MallCode: "M001", NameCN: "上海某商场",
			NameEN: "Shanghai Mall", Province: "上海市", City: "上海市", District: "浦东新区",
			Timezone: "Asia/Shanghai", WeatherEnabled: true, ContactPhone: "secret",
		},
		{BaseModel: model.BaseModel{ID: 8}, MallCode: "M002", WeatherEnabled: true},
	}}
	permissions := &fakeOpenWeatherMallPermissionReader{allowed: true}
	service := newOpenWeatherMallQueryService(malls, permissions, time.Now)
	result, err := service.Query(t.Context(), 17, requestbody.OpenWeatherMallQueryRequest{PageSize: 1})
	if err != nil {
		t.Fatalf("Query() error=%v", err)
	}
	if permissions.permission != model.PermissionWeatherRead || malls.calls != 1 || malls.afterID != 0 || malls.limit != 2 {
		t.Fatalf("permission=%q calls=%d afterID=%d limit=%d", permissions.permission, malls.calls, malls.afterID, malls.limit)
	}
	if len(result.Items) != 1 || result.Items[0].ID != 7 || result.Items[0].TimeZone != "Asia/Shanghai" ||
		!result.Pagination.HasMore || result.Pagination.NextCursor == "" {
		t.Fatalf("result=%+v", result)
	}
	payload, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	if strings.Contains(string(payload), "secret") || strings.Contains(string(payload), "contactPhone") {
		t.Fatalf("response leaked internal field: %s", payload)
	}
	cursor, err := decodeOpenWeatherMallCursor(result.Pagination.NextCursor)
	if err != nil || cursor.ID != 7 || cursor.Version != 1 {
		t.Fatalf("cursor=%+v error=%v", cursor, err)
	}
}

func TestOpenWeatherMallQueryServiceContinuesFromCursor(t *testing.T) {
	cursor, err := encodeOpenWeatherMallCursor(7)
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
