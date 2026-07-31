package data_ctrl

import (
	"encoding/json"
	"strings"
	"testing"

	"gin-biz-web-api/internal/service/data_svc"
	"gin-biz-web-api/model"
)

func TestRedactConfigJSON_RedactsPipelineSensitiveKeys(t *testing.T) {
	secretValues := map[string]string{
		"Authorization": "Bearer pipeline-authorization-value",
		"token":         "pipeline-token-value",
		"secret":        "pipeline-secret-value",
		"password":      "pipeline-password-value",
		"api_key":       "pipeline-api-key-value",
		"private_key":   "pipeline-private-key-value",
	}

	config, err := json.Marshal(map[string]any{
		"headers": secretValues,
		"normal": map[string]any{
			"endpoint": "https://pipeline.example.test/v1/orders",
			"retries":  3,
			"enabled":  true,
		},
		"nested": []any{map[string]any{"token": "nested-pipeline-token"}},
	})
	if err != nil {
		t.Fatalf("marshal pipeline config: %v", err)
	}

	redacted, hasSecret := redactConfigJSON(string(config))
	if !hasSecret {
		t.Fatal("hasSecret = false, want true")
	}
	for name, secretValue := range secretValues {
		if strings.Contains(redacted, secretValue) {
			t.Fatalf("redacted pipeline config leaked %s value: %s", name, redacted)
		}
	}
	if strings.Contains(redacted, "nested-pipeline-token") {
		t.Fatalf("redacted pipeline config leaked nested token: %s", redacted)
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(redacted), &decoded); err != nil {
		t.Fatalf("redacted pipeline config must remain valid JSON: %v", err)
	}
	headers, ok := decoded["headers"].(map[string]any)
	if !ok {
		t.Fatalf("headers = %#v, want object", decoded["headers"])
	}
	for name := range secretValues {
		if headers[name] != "[已隐藏]" {
			t.Errorf("headers[%q] = %#v, want redacted marker", name, headers[name])
		}
	}
	normal, ok := decoded["normal"].(map[string]any)
	if !ok {
		t.Fatalf("normal = %#v, want object", decoded["normal"])
	}
	if normal["endpoint"] != "https://pipeline.example.test/v1/orders" || normal["retries"] != float64(3) || normal["enabled"] != true {
		t.Errorf("normal values changed after redaction: %#v", normal)
	}
}

func TestRedactConfigJSON_PreservesPipelineStructure(t *testing.T) {
	redacted, hasSecret := redactConfigJSON(`{"steps":[{"code":"fetch","headers":{"Content-Type":"application/json","Authorization":"Bearer not-for-response"}},{"code":"transform","mapping":{"table_name":"clean_orders"}}]}`)
	if !hasSecret {
		t.Fatal("hasSecret = false, want true")
	}
	if strings.Contains(redacted, "Bearer not-for-response") {
		t.Fatalf("redacted pipeline structure leaked authorization value: %s", redacted)
	}

	var decoded struct {
		Steps []struct {
			Code    string         `json:"code"`
			Headers map[string]any `json:"headers"`
			Mapping map[string]any `json:"mapping"`
		} `json:"steps"`
	}
	if err := json.Unmarshal([]byte(redacted), &decoded); err != nil {
		t.Fatalf("redacted pipeline structure must remain valid JSON: %v", err)
	}
	if len(decoded.Steps) != 2 || decoded.Steps[0].Code != "fetch" || decoded.Steps[1].Code != "transform" {
		t.Fatalf("pipeline steps changed after redaction: %#v", decoded.Steps)
	}
	if decoded.Steps[0].Headers["Content-Type"] != "application/json" || decoded.Steps[0].Headers["Authorization"] != "[已隐藏]" {
		t.Errorf("fetch headers = %#v, want regular header retained and authorization redacted", decoded.Steps[0].Headers)
	}
	if decoded.Steps[1].Mapping["table_name"] != "clean_orders" {
		t.Errorf("mapping changed after redaction: %#v", decoded.Steps[1].Mapping)
	}
}

func TestSafeMethodParamValue_RedactsSecretsAndBoundsRegularValues(t *testing.T) {
	longValue := strings.Repeat("safe", pipelineResponseTextLimit)
	tests := []struct {
		name  string
		param model.MethodParam
		want  string
	}{
		{
			name:  "explicit secret flag",
			param: model.MethodParam{Name: "merchant_id", Value: "secret-flag-value", Secret: true},
			want:  "[已隐藏]",
		},
		{
			name:  "secret value source",
			param: model.MethodParam{Name: "merchant_id", Value: "secret-source-value", ValueSource: "SeCrEt"},
			want:  "[已隐藏]",
		},
		{
			name:  "sensitive name",
			param: model.MethodParam{Name: "Authorization", Value: "authorization-param-value"},
			want:  "[已隐藏]",
		},
		{
			name:  "regular value",
			param: model.MethodParam{Name: "page_size", Value: "100"},
			want:  "100",
		},
		{
			name:  "long regular value",
			param: model.MethodParam{Name: "template", Value: longValue},
			want:  longValue[:pipelineResponseTextLimit] + "…",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := safeMethodParamValue(tt.param); got != tt.want {
				t.Errorf("safeMethodParamValue() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPipelineResponses_RedactSensitiveNamedValues(t *testing.T) {
	secrets := []string{
		"pipeline-authorization-value",
		"pipeline-token-value",
		"pipeline-secret-value",
		"pipeline-password-value",
		"pipeline-api-key-value",
		"pipeline-private-key-value",
	}
	sensitiveConfig := `{
		"headers":[{"name":"Authorization","value":"pipeline-authorization-value","secret":true}],
		"query_params":[{"name":"token","value":"pipeline-token-value"}],
		"params":[
			{"name":"secret","value":"pipeline-secret-value"},
			{"name":"password","value":"pipeline-password-value"},
			{"name":"api_key","value":"pipeline-api-key-value"},
			{"name":"private_key","value":"pipeline-private-key-value"}
		],
		"endpoint":"https://pipeline.example.test/v1/orders",
		"retries":3
	}`

	detail := safeMethodStepDetail(data_svc.MethodStepDetail{
		Step: model.MethodStep{Code: "fetch_orders", GeneratedConfigJSON: sensitiveConfig},
		Params: []model.MethodParam{
			{Name: "merchant_id", Value: "visible-value"},
			{Name: "request_key", Value: "hidden-by-flag", Secret: true},
		},
	})
	assertPipelineResponseDoesNotLeak(t, detail.Step.GeneratedConfigJSON, secrets...)
	if detail.Params[0].Value != "visible-value" || detail.Params[1].Value != "[已隐藏]" {
		t.Errorf("safe method params = %#v, want regular value retained and secret value hidden", detail.Params)
	}

	stage := safeStageGeneratedConfig(model.StageGeneratedConfig{GeneratedConfigJSON: sensitiveConfig})
	assertPipelineResponseDoesNotLeak(t, stage.GeneratedConfigJSON, secrets...)

	stepRuns := safeStepRuns([]model.StepRun{{
		InputJSON:           sensitiveConfig,
		OutputJSON:          `{"result":"ok","private_key":"pipeline-private-key-value"}`,
		GeneratedConfigJSON: sensitiveConfig,
	}})
	if len(stepRuns) != 1 {
		t.Fatalf("safeStepRuns() length = %d, want 1", len(stepRuns))
	}
	assertPipelineResponseDoesNotLeak(t, stepRuns[0].InputJSON, secrets...)
	assertPipelineResponseDoesNotLeak(t, stepRuns[0].OutputJSON, secrets...)
	assertPipelineResponseDoesNotLeak(t, stepRuns[0].GeneratedConfigJSON, secrets...)

	preview := safePipelinePreview(map[string]interface{}{
		"steps": []interface{}{map[string]interface{}{
			"headers": []interface{}{map[string]interface{}{
				"name": "Authorization", "value": "pipeline-authorization-value", "secret": true,
			}},
			"endpoint": "https://pipeline.example.test/v1/orders",
		}},
	})
	previewJSON, err := json.Marshal(preview)
	if err != nil {
		t.Fatalf("marshal pipeline preview: %v", err)
	}
	assertPipelineResponseDoesNotLeak(t, string(previewJSON), "pipeline-authorization-value")
	if !strings.Contains(string(previewJSON), "https://pipeline.example.test/v1/orders") {
		t.Errorf("pipeline preview lost regular value: %s", previewJSON)
	}
}

func TestRedactPipelineJSON_BoundsLargeResponse(t *testing.T) {
	value := `{"result":"` + strings.Repeat("x", pipelineResponseTextLimit*2) + `"}`
	redacted := redactPipelineJSON(value)
	var decoded map[string]string
	if err := json.Unmarshal([]byte(redacted), &decoded); err != nil {
		t.Fatalf("redacted pipeline response must remain valid JSON: %v", err)
	}
	if got, max := len(decoded["result"]), pipelineResponseTextLimit+len("…"); got > max {
		t.Errorf("redacted result length = %d, want <= %d", got, max)
	}
}

func TestSafePipelinePreview_BoundsRegularText(t *testing.T) {
	preview := safePipelinePreview(map[string]interface{}{
		"status":  "ready",
		"payload": strings.Repeat("x", pipelineResponseTextLimit*2),
	})
	payload, ok := preview["payload"].(string)
	if !ok {
		t.Fatalf("preview payload = %#v, want string", preview["payload"])
	}
	if got, max := len(payload), pipelineResponseTextLimit+len("…"); got > max {
		t.Errorf("preview payload length = %d, want <= %d", got, max)
	}
	if preview["status"] != "ready" {
		t.Errorf("preview status = %#v, want ready", preview["status"])
	}
}

func assertPipelineResponseDoesNotLeak(t *testing.T, value string, secrets ...string) {
	t.Helper()
	for _, secret := range secrets {
		if strings.Contains(value, secret) {
			t.Errorf("pipeline response leaked %q in %s", secret, value)
		}
	}
}
