package data_ctrl

import (
	"testing"

	"gin-biz-web-api/internal/requestbody"
)

func TestProcessedDataListQueryValid(t *testing.T) {
	minimum := 40.0
	maximum := 80.0
	tests := []struct {
		name string
		req  requestbody.ProcessedDataListQueryRequest
		want bool
	}{
		{name: "empty", want: true},
		{name: "valid quality", req: requestbody.ProcessedDataListQueryRequest{MinQuality: &minimum, MaxQuality: &maximum}, want: true},
		{name: "inverted quality", req: requestbody.ProcessedDataListQueryRequest{MinQuality: &maximum, MaxQuality: &minimum}, want: false},
		{name: "valid time", req: requestbody.ProcessedDataListQueryRequest{CreatedFrom: 10, CreatedTo: 20}, want: true},
		{name: "inverted time", req: requestbody.ProcessedDataListQueryRequest{CreatedFrom: 20, CreatedTo: 10}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := processedDataListQueryValid(tt.req); got != tt.want {
				t.Fatalf("processedDataListQueryValid() = %v, want %v", got, tt.want)
			}
		})
	}
}
