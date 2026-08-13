package requestbody

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin/binding"
)

func TestEnabledUpdateRequestRequiresExplicitBoolean(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		want     bool
		wantFail bool
	}{
		{name: "enabled", body: `{"enabled":true}`, want: true},
		{name: "disabled", body: `{"enabled":false}`},
		{name: "missing", body: `{}`, wantFail: true},
		{name: "wrong type", body: `{"enabled":"false"}`, wantFail: true},
		{name: "invalid json", body: `{`, wantFail: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest("PATCH", "/enabled", strings.NewReader(tt.body))
			request.Header.Set("Content-Type", "application/json")
			var value EnabledUpdateRequest
			err := binding.JSON.Bind(request, &value)
			if tt.wantFail {
				if err == nil {
					t.Fatal("Bind() error = nil")
				}
				return
			}
			if err != nil || value.Enabled == nil || *value.Enabled != tt.want {
				t.Fatalf("Bind() enabled = %v, error = %v", value.Enabled, err)
			}
		})
	}
}
