package data_ctrl

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"gin-biz-web-api/internal/service/data_svc"
)

func TestBojunOrderBackfillControllerConfirmEnqueuesBackgroundTask(t *testing.T) {
	gin.SetMode(gin.TestMode)
	taskService := &fakeBojunOrderTaskService{
		result: &data_svc.LegacyTaskRunResult{ID: "task-17", Queue: "default", Type: "bojun:order:fetch"},
	}
	controller := newBojunOrderBackfillController(nil, taskService)
	router := gin.New()
	router.POST("/confirm", controller.Confirm)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/confirm",
		strings.NewReader(`{"start_time":"2026-08-10T00:00","end_time":"2026-08-15T00:00"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s, want %d", recorder.Code, recorder.Body.String(), http.StatusAccepted)
	}
	if taskService.code != bojunOrderBackfillTaskCode ||
		taskService.payload["source_mode"] != "api" ||
		taskService.payload["start_time"] != "2026-08-10 00:00:00" ||
		taskService.payload["end_time"] != "2026-08-15 00:00:00" {
		t.Fatalf("enqueued code=%q payload=%v", taskService.code, taskService.payload)
	}

	var response struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			Result data_svc.LegacyTaskRunResult `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Code != http.StatusOK || response.Msg != "伯俊补拉任务已投递" || response.Data.Result.ID != "task-17" {
		t.Fatalf("response=%+v", response)
	}
}

func TestBojunOrderBackfillControllerConfirmReturnsEnqueueFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	controller := newBojunOrderBackfillController(nil, &fakeBojunOrderTaskService{err: errors.New("queue unavailable")})
	router := gin.New()
	router.POST("/confirm", controller.Confirm)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/confirm",
		strings.NewReader(`{"start_time":"2026-08-10T00:00","end_time":"2026-08-15T00:00"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s, want %d", recorder.Code, recorder.Body.String(), http.StatusInternalServerError)
	}
}

func TestBojunOrderBackfillControllerConfirmRejectsInvalidRangeBeforeEnqueue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	taskService := &fakeBojunOrderTaskService{}
	controller := newBojunOrderBackfillController(nil, taskService)
	router := gin.New()
	router.POST("/confirm", controller.Confirm)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/confirm",
		strings.NewReader(`{"start_time":"2026-08-15T00:00","end_time":"2026-08-10T00:00"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s, want %d", recorder.Code, recorder.Body.String(), http.StatusBadRequest)
	}
	if taskService.payload != nil {
		t.Fatalf("unexpected enqueue payload=%v", taskService.payload)
	}
}

type fakeBojunOrderTaskService struct {
	code    string
	payload map[string]interface{}
	result  *data_svc.LegacyTaskRunResult
	err     error
}

func (service *fakeBojunOrderTaskService) Enqueue(
	_ context.Context,
	code string,
	payload map[string]interface{},
) (*data_svc.LegacyTaskRunResult, error) {
	service.code = code
	service.payload = payload
	return service.result, service.err
}
