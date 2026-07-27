package data_svc

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"gin-biz-web-api/connector/caiyun"
	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/job"
	"gin-biz-web-api/model"
)

const maxMallWeatherCorrelationIDLength = 128

type MallWeatherFetchRunDTO struct {
	RunUUID                 string                  `json:"runUuid"`
	CorrelationID           string                  `json:"correlationId"`
	Provider                string                  `json:"provider"`
	EndpointKind            string                  `json:"endpointKind"`
	TaskKind                string                  `json:"taskKind"`
	RequestedHourlySteps    int                     `json:"requestedHourlySteps"`
	RequestedDailySteps     int                     `json:"requestedDailySteps"`
	AttemptCount            int                     `json:"attemptCount"`
	Status                  string                  `json:"status"`
	StartedAtUTC            *time.Time              `json:"startedAtUtc,omitempty"`
	StartedAtLocal          *string                 `json:"startedAtLocal,omitempty"`
	FinishedAtUTC           *time.Time              `json:"finishedAtUtc,omitempty"`
	FinishedAtLocal         *string                 `json:"finishedAtLocal,omitempty"`
	DurationMS              int64                   `json:"durationMs"`
	HTTPStatus              *int                    `json:"httpStatus,omitempty"`
	ProviderStatus          string                  `json:"providerStatus,omitempty"`
	ProviderServerTimeUTC   *time.Time              `json:"providerServerTimeUtc,omitempty"`
	ProviderServerTimeLocal *string                 `json:"providerServerTimeLocal,omitempty"`
	ResponseChecksum        string                  `json:"responseChecksum,omitempty"`
	RowCounts               map[string]int64        `json:"rowCounts"`
	ParseWarnings           []MallWeatherWarningDTO `json:"parseWarnings"`
	ErrorClass              string                  `json:"errorClass,omitempty"`
	ErrorCode               string                  `json:"errorCode,omitempty"`
	ErrorMessageSafe        string                  `json:"errorMessageSafe,omitempty"`
	ParserVersion           string                  `json:"parserVersion,omitempty"`
	CreatedAtUTC            time.Time               `json:"createdAtUtc"`
	CreatedAtLocal          string                  `json:"createdAtLocal"`
	UpdatedAtUTC            time.Time               `json:"updatedAtUtc"`
	UpdatedAtLocal          string                  `json:"updatedAtLocal"`
}

type MallWeatherFetchRunMeta struct {
	TimeZone string `json:"timeZone"`
}

type MallWeatherFetchRunResult struct {
	Items      []MallWeatherFetchRunDTO `json:"items"`
	Meta       MallWeatherFetchRunMeta  `json:"meta"`
	Pagination MallWeatherPagination    `json:"pagination"`
}

type weatherFetchRunCursor struct {
	Version         int   `json:"v"`
	CreatedAtUnixMS int64 `json:"createdAtUnixMs"`
	ID              uint  `json:"id"`
}

func (service *MallWeatherQueryService) FetchRuns(ctx context.Context, actorUserID, mallID uint, request requestbody.MallWeatherFetchRunQueryRequest) (*MallWeatherFetchRunResult, error) {
	if service == nil || ctx == nil || mallID == 0 {
		return nil, fmt.Errorf("%w: invalid request", ErrMallWeatherInvalidQuery)
	}
	if err := service.authorize(ctx, actorUserID); err != nil {
		return nil, err
	}
	mall, err := service.malls.FindByID(ctx, mallID)
	if err != nil {
		return nil, err
	}
	location, normalized, err := normalizeFetchRunWeatherRequest(request, mall)
	if err != nil {
		return nil, err
	}
	cursor, err := decodeWeatherFetchRunCursor(normalized.Cursor)
	if err != nil {
		return nil, err
	}
	query := data_dao.FetchRunQuery{
		MallID: mallID, StartUTC: normalized.StartUTC, EndUTC: normalized.EndUTC,
		CorrelationID: normalized.CorrelationID, TaskKind: normalized.TaskKind,
		EndpointKind: normalized.EndpointKind, Status: normalized.Status,
		Limit: normalized.PageSize + 1,
	}
	if cursor != nil {
		createdAt := time.UnixMilli(cursor.CreatedAtUnixMS).UTC()
		query.AfterCreatedAt = &createdAt
		query.AfterID = cursor.ID
	}
	rows, err := service.weather.QueryFetchRuns(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("mall weather query: fetch runs: %w", err)
	}
	hasMore := len(rows) > normalized.PageSize
	if hasMore {
		rows = rows[:normalized.PageSize]
	}
	items := make([]MallWeatherFetchRunDTO, len(rows))
	for index := range rows {
		items[index], err = fetchRunWeatherDTO(&rows[index], location)
		if err != nil {
			return nil, err
		}
	}
	result := &MallWeatherFetchRunResult{
		Items: items, Meta: MallWeatherFetchRunMeta{TimeZone: location.String()},
		Pagination: MallWeatherPagination{PageSize: normalized.PageSize},
	}
	if hasMore && len(rows) > 0 {
		result.Pagination.NextCursor, err = encodeWeatherFetchRunCursor(&rows[len(rows)-1])
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func normalizeFetchRunWeatherRequest(request requestbody.MallWeatherFetchRunQueryRequest, mall *model.Mall) (*time.Location, requestbody.MallWeatherFetchRunQueryRequest, error) {
	if mall == nil || mall.ID == 0 || request.StartUTC.IsZero() || request.EndUTC.IsZero() || !request.StartUTC.Before(request.EndUTC) ||
		request.EndUTC.Sub(request.StartUTC) > maxWeatherQueryRange {
		return nil, request, fmt.Errorf("%w: invalid fetch run range", ErrMallWeatherInvalidQuery)
	}
	zoneName := strings.TrimSpace(request.TimeZone)
	if zoneName == "" {
		zoneName = strings.TrimSpace(mall.Timezone)
	}
	if zoneName == "" {
		zoneName = "Asia/Shanghai"
	}
	location, err := time.LoadLocation(zoneName)
	if err != nil {
		return nil, request, fmt.Errorf("%w: invalid time zone", ErrMallWeatherInvalidQuery)
	}
	request.StartUTC, request.EndUTC, request.TimeZone = request.StartUTC.UTC(), request.EndUTC.UTC(), location.String()
	request.CorrelationID = strings.TrimSpace(request.CorrelationID)
	if len(request.CorrelationID) > maxMallWeatherCorrelationIDLength || !utf8.ValidString(request.CorrelationID) ||
		strings.ContainsAny(request.CorrelationID, "\x00\r\n") {
		return nil, request, fmt.Errorf("%w: invalid correlation id", ErrMallWeatherInvalidQuery)
	}
	request.TaskKind, err = normalizeFetchRunTaskKind(request.TaskKind)
	if err != nil {
		return nil, request, err
	}
	request.EndpointKind, err = normalizeFetchRunEndpointKind(request.EndpointKind)
	if err != nil {
		return nil, request, err
	}
	request.Status = strings.ToLower(strings.TrimSpace(request.Status))
	switch request.Status {
	case "", "pending", "running", "success", "partial_success", "failed":
	default:
		return nil, request, fmt.Errorf("%w: invalid fetch run status", ErrMallWeatherInvalidQuery)
	}
	if request.PageSize == 0 {
		request.PageSize = defaultWeatherQueryPageSize
	}
	if request.PageSize < 1 || request.PageSize > maxWeatherQueryPageSize || len(request.Cursor) > maxWeatherCursorLength {
		return nil, request, fmt.Errorf("%w: invalid pagination", ErrMallWeatherInvalidQuery)
	}
	return location, request, nil
}

func normalizeFetchRunTaskKind(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "":
		return "", nil
	case "fast", job.TypeMallWeatherFast:
		return "fast", nil
	case "full", job.TypeMallWeatherFull:
		return "full", nil
	case "lifeindex", "life_index", job.TypeMallWeatherLifeIndex:
		return "lifeindex", nil
	case "repair", job.TypeMallWeatherRepair:
		return "repair", nil
	case "manual", job.TypeMallWeatherManual:
		return "manual", nil
	default:
		return "", fmt.Errorf("%w: invalid fetch run task kind", ErrMallWeatherInvalidQuery)
	}
}

func normalizeFetchRunEndpointKind(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "":
		return "", nil
	case caiyun.EndpointWeatherV26:
		return caiyun.EndpointWeatherV26, nil
	case caiyun.EndpointLifeIndexV3, "v3_lifeindex":
		return caiyun.EndpointLifeIndexV3, nil
	default:
		return "", fmt.Errorf("%w: invalid fetch run endpoint kind", ErrMallWeatherInvalidQuery)
	}
}

func fetchRunWeatherDTO(row *model.MallWeatherFetchRun, location *time.Location) (MallWeatherFetchRunDTO, error) {
	if row == nil || row.ID == 0 || strings.TrimSpace(row.RunUUID) == "" || row.MallID == 0 || row.Provider == "" ||
		strings.TrimSpace(row.TaskWindow) == "" || len(row.TaskWindow) > maxMallWeatherCorrelationIDLength ||
		row.EndpointKind == "" || row.TaskKind == "" || row.Status == "" || row.CreatedAt.IsZero() || row.UpdatedAt.IsZero() ||
		row.RequestedHourlySteps < 0 || row.RequestedDailySteps < 0 || row.AttemptCount < 0 || row.DurationMS < 0 || location == nil {
		return MallWeatherFetchRunDTO{}, fmt.Errorf("mall weather query: invalid fetch run row")
	}
	rowCounts := make(map[string]int64)
	if strings.TrimSpace(string(row.RowCountsJSON)) != "" {
		if err := json.Unmarshal([]byte(row.RowCountsJSON), &rowCounts); err != nil {
			return MallWeatherFetchRunDTO{}, fmt.Errorf("mall weather query: decode fetch run row counts: %w", err)
		}
	}
	for key, count := range rowCounts {
		if strings.TrimSpace(key) == "" || len(key) > 128 || count < 0 {
			return MallWeatherFetchRunDTO{}, fmt.Errorf("mall weather query: invalid fetch run row counts")
		}
	}
	if len(rowCounts) > 64 {
		return MallWeatherFetchRunDTO{}, fmt.Errorf("mall weather query: invalid fetch run row counts")
	}
	warnings, err := weatherQualityWarnings(row.ParseWarningsJSON)
	if err != nil {
		return MallWeatherFetchRunDTO{}, err
	}
	dto := MallWeatherFetchRunDTO{
		RunUUID: row.RunUUID, CorrelationID: row.TaskWindow,
		Provider: strings.ToUpper(row.Provider), EndpointKind: row.EndpointKind,
		TaskKind: publicFetchRunTaskKind(row.TaskKind), RequestedHourlySteps: row.RequestedHourlySteps,
		RequestedDailySteps: row.RequestedDailySteps, AttemptCount: row.AttemptCount, Status: strings.ToUpper(row.Status),
		DurationMS: row.DurationMS, HTTPStatus: row.HTTPStatus, ProviderStatus: row.ProviderStatus,
		ResponseChecksum: row.ResponseChecksum, RowCounts: rowCounts, ParseWarnings: warnings,
		ErrorClass: row.ErrorClass, ErrorCode: row.ErrorCode, ErrorMessageSafe: row.ErrorMessageSafe, ParserVersion: row.ParserVersion,
		CreatedAtUTC: row.CreatedAt.UTC(), CreatedAtLocal: formatWeatherLocalTime(row.CreatedAt, location),
		UpdatedAtUTC: row.UpdatedAt.UTC(), UpdatedAtLocal: formatWeatherLocalTime(row.UpdatedAt, location),
	}
	setFetchRunOptionalTime(row.StartedAt, location, &dto.StartedAtUTC, &dto.StartedAtLocal)
	setFetchRunOptionalTime(row.FinishedAt, location, &dto.FinishedAtUTC, &dto.FinishedAtLocal)
	setFetchRunOptionalTime(row.ProviderServerTime, location, &dto.ProviderServerTimeUTC, &dto.ProviderServerTimeLocal)
	return dto, nil
}

func setFetchRunOptionalTime(value *time.Time, location *time.Location, utc **time.Time, local **string) {
	if value == nil {
		return
	}
	utcValue := value.UTC()
	localValue := formatWeatherLocalTime(utcValue, location)
	*utc, *local = &utcValue, &localValue
}

func publicFetchRunTaskKind(value string) string {
	switch value {
	case "fast", job.TypeMallWeatherFast:
		return "FAST"
	case "full", job.TypeMallWeatherFull:
		return "FULL"
	case "lifeindex", job.TypeMallWeatherLifeIndex:
		return "LIFEINDEX"
	case "repair", job.TypeMallWeatherRepair:
		return "REPAIR"
	case "manual", job.TypeMallWeatherManual:
		return "MANUAL"
	default:
		return strings.ToUpper(value)
	}
}

func encodeWeatherFetchRunCursor(row *model.MallWeatherFetchRun) (string, error) {
	if row == nil || row.ID == 0 || row.CreatedAt.IsZero() {
		return "", fmt.Errorf("mall weather query: invalid fetch run cursor row")
	}
	payload, err := json.Marshal(weatherFetchRunCursor{Version: 1, CreatedAtUnixMS: row.CreatedAt.UTC().UnixMilli(), ID: row.ID})
	if err != nil {
		return "", fmt.Errorf("mall weather query: encode fetch run cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func decodeWeatherFetchRunCursor(value string) (*weatherFetchRunCursor, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	if len(value) > maxWeatherCursorLength {
		return nil, fmt.Errorf("%w: invalid cursor", ErrMallWeatherInvalidQuery)
	}
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid cursor", ErrMallWeatherInvalidQuery)
	}
	var cursor weatherFetchRunCursor
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cursor); err != nil || cursor.Version != 1 || cursor.ID == 0 ||
		cursor.CreatedAtUnixMS < minWeatherCursorUnixMS || cursor.CreatedAtUnixMS >= maxWeatherCursorUnixMS {
		return nil, fmt.Errorf("%w: invalid cursor", ErrMallWeatherInvalidQuery)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%w: invalid cursor", ErrMallWeatherInvalidQuery)
	}
	return &cursor, nil
}
