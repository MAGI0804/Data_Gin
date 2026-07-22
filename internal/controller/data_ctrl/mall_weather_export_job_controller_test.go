package data_ctrl

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gin-biz-web-api/constant"
	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/internal/service/data_svc"

	"github.com/gin-gonic/gin"
)

const mallWeatherExportJobTestUUID = "00000000-0000-4000-8000-000000000017"

func TestMallWeatherExportJobControllerCreatesAcceptedJob(t *testing.T) {
	service := fakeMallWeatherExportJobControllerService{
		create: func(
			_ context.Context,
			actor uint,
			key string,
			request requestbody.MallWeatherExportCreateRequest,
		) (*data_svc.MallWeatherExportCreateResult, bool, error) {
			if actor != 17 || key != "weather-export-key-1234" || request.ProfileID != 9 ||
				request.ExpectedProfileVersion == nil || *request.ExpectedProfileVersion != 3 {
				t.Fatalf("actor=%d key=%q request=%+v", actor, key, request)
			}
			return &data_svc.MallWeatherExportCreateResult{
				JobID: mallWeatherExportJobTestUUID, Status: "PENDING", ProfileID: 9, ProfileVersion: 3,
			}, true, nil
		},
	}
	recorder := performMallWeatherExportJobRequest(
		t,
		service,
		http.MethodPost,
		"/api/v1/weather-exports",
		`{"profileId":9,"expectedProfileVersion":3}`,
		"weather-export-key-1234",
	)
	if recorder.Code != http.StatusAccepted || recorder.Header().Get("Idempotency-Replayed") != "true" ||
		!strings.Contains(recorder.Body.String(), `"jobId":"`+mallWeatherExportJobTestUUID+`"`) {
		t.Fatalf("status=%d headers=%v body=%s", recorder.Code, recorder.Header(), recorder.Body.String())
	}
}

func TestMallWeatherExportJobControllerGetsActorScopedJob(t *testing.T) {
	service := fakeMallWeatherExportJobControllerService{
		get: func(_ context.Context, actor uint, jobUUID string) (*data_svc.MallWeatherExportJobDTO, error) {
			if actor != 17 || jobUUID != mallWeatherExportJobTestUUID {
				t.Fatalf("actor=%d jobUUID=%q", actor, jobUUID)
			}
			return &data_svc.MallWeatherExportJobDTO{JobID: jobUUID, Status: "RUNNING"}, nil
		},
	}
	recorder := performMallWeatherExportJobRequest(
		t,
		service,
		http.MethodGet,
		"/api/v1/weather-exports/"+mallWeatherExportJobTestUUID,
		"",
		"",
	)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"status":"RUNNING"`) {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestMallWeatherExportJobControllerReturnsSignedDownload(t *testing.T) {
	service := fakeMallWeatherExportJobControllerService{
		download: func(_ context.Context, actor uint, jobUUID string) (*data_svc.MallWeatherExportDownloadResult, error) {
			if actor != 17 || jobUUID != mallWeatherExportJobTestUUID {
				t.Fatalf("actor=%d job=%s", actor, jobUUID)
			}
			return &data_svc.MallWeatherExportDownloadResult{URL: "https://signed.example/result"}, nil
		},
	}
	recorder := performMallWeatherExportJobRequest(
		t,
		service,
		http.MethodGet,
		"/api/v1/weather-exports/"+mallWeatherExportJobTestUUID+"/download",
		"",
		"",
	)
	if recorder.Code != http.StatusOK || strings.Contains(recorder.Body.String(), "object_key") ||
		!strings.Contains(recorder.Body.String(), "https://signed.example/result") {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestMallWeatherExportJobControllerRejectsUnknownJSONField(t *testing.T) {
	calls := 0
	service := fakeMallWeatherExportJobControllerService{
		create: func(
			context.Context,
			uint,
			string,
			requestbody.MallWeatherExportCreateRequest,
		) (*data_svc.MallWeatherExportCreateResult, bool, error) {
			calls++
			return nil, false, nil
		},
	}
	recorder := performMallWeatherExportJobRequest(
		t,
		service,
		http.MethodPost,
		"/api/v1/weather-exports",
		`{"profileId":9,"secret":"x"}`,
		"weather-export-key-1234",
	)
	if recorder.Code != http.StatusUnprocessableEntity || calls != 0 ||
		strings.Contains(recorder.Body.String(), "secret") {
		t.Fatalf("status=%d calls=%d body=%s", recorder.Code, calls, recorder.Body.String())
	}
}

func TestMallWeatherExportJobControllerMapsSafeErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{name: "forbidden", err: data_svc.ErrMallForbidden, wantStatus: http.StatusForbidden},
		{name: "profile not found", err: data_dao.ErrMallWeatherExportProfileNotFound, wantStatus: http.StatusNotFound},
		{name: "job not found", err: data_dao.ErrMallWeatherExportJobNotFound, wantStatus: http.StatusNotFound},
		{name: "idempotency conflict", err: data_svc.ErrMallIdempotencyConflict, wantStatus: http.StatusConflict},
		{name: "profile conflict", err: data_svc.ErrMallWeatherExportProfileConflict, wantStatus: http.StatusConflict},
		{name: "not ready", err: data_svc.ErrMallWeatherExportNotReady, wantStatus: http.StatusConflict},
		{name: "expired", err: data_svc.ErrMallWeatherExportExpired, wantStatus: http.StatusConflict},
		{name: "invalid", err: data_svc.ErrMallWeatherExportInvalid, wantStatus: http.StatusUnprocessableEntity},
		{name: "too large", err: data_svc.ErrMallWeatherExportTooLarge, wantStatus: http.StatusUnprocessableEntity},
		{name: "internal", err: errors.New("database password=secret"), wantStatus: http.StatusInternalServerError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := fakeMallWeatherExportJobControllerService{
				get: func(context.Context, uint, string) (*data_svc.MallWeatherExportJobDTO, error) {
					return nil, test.err
				},
			}
			recorder := performMallWeatherExportJobRequest(
				t,
				service,
				http.MethodGet,
				"/api/v1/weather-exports/"+mallWeatherExportJobTestUUID,
				"",
				"",
			)
			if recorder.Code != test.wantStatus || strings.Contains(recorder.Body.String(), "password") ||
				strings.Contains(recorder.Body.String(), "secret") {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func performMallWeatherExportJobRequest(
	t *testing.T,
	service MallWeatherExportJobServiceAPI,
	method string,
	path string,
	body string,
	idempotencyKey string,
) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(constant.CurrentUserID, "17")
		c.Next()
	})
	controller := NewMallWeatherExportJobControllerWithService(service)
	router.POST("/api/v1/weather-exports", controller.Create)
	router.GET("/api/v1/weather-exports/:job_id", controller.Get)
	router.GET("/api/v1/weather-exports/:job_id/download", controller.Download)
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	if idempotencyKey != "" {
		request.Header.Set("Idempotency-Key", idempotencyKey)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

type fakeMallWeatherExportJobControllerService struct {
	create func(
		context.Context,
		uint,
		string,
		requestbody.MallWeatherExportCreateRequest,
	) (*data_svc.MallWeatherExportCreateResult, bool, error)
	get      func(context.Context, uint, string) (*data_svc.MallWeatherExportJobDTO, error)
	download func(context.Context, uint, string) (*data_svc.MallWeatherExportDownloadResult, error)
}

func (service fakeMallWeatherExportJobControllerService) Create(
	ctx context.Context,
	actor uint,
	key string,
	request requestbody.MallWeatherExportCreateRequest,
) (*data_svc.MallWeatherExportCreateResult, bool, error) {
	if service.create == nil {
		panic("unexpected Create call")
	}
	return service.create(ctx, actor, key, request)
}

func (service fakeMallWeatherExportJobControllerService) Get(
	ctx context.Context,
	actor uint,
	jobUUID string,
) (*data_svc.MallWeatherExportJobDTO, error) {
	if service.get == nil {
		panic("unexpected Get call")
	}
	return service.get(ctx, actor, jobUUID)
}

func (service fakeMallWeatherExportJobControllerService) Download(
	ctx context.Context,
	actor uint,
	jobUUID string,
) (*data_svc.MallWeatherExportDownloadResult, error) {
	if service.download == nil {
		return nil, errors.New("download not configured")
	}
	return service.download(ctx, actor, jobUUID)
}
