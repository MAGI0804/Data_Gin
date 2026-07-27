package storage

import (
	"net/url"
	"strings"
	"testing"
	"time"

	alioss "github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"
)

func TestNormalizeOSSRegion(t *testing.T) {
	tests := map[string]string{
		"cn-shanghai":                                   "cn-shanghai",
		"oss-cn-shanghai":                               "cn-shanghai",
		" https://oss-cn-shanghai.aliyuncs.com ":        "cn-shanghai",
		"https://oss-cn-shanghai-internal.aliyuncs.com": "cn-shanghai",
		"http://oss-cn-hangzhou.aliyuncs.com":           "cn-hangzhou",
		"":                                              "",
	}

	for input, want := range tests {
		if got := normalizeOSSRegion(input); got != want {
			t.Fatalf("normalizeOSSRegion(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestOSSClientPublicURLUsesNormalizedRegionFallback(t *testing.T) {
	client := &OSSClient{
		cfg: OSSConfig{
			Region: "oss-cn-shanghai",
			Bucket: "youlan-warehouse",
		},
	}

	got := client.PublicURL("data-warehouse/result.xlsx")
	want := "https://youlan-warehouse.oss-cn-shanghai.aliyuncs.com/data-warehouse/result.xlsx"
	if got != want {
		t.Fatalf("PublicURL() = %q, want %q", got, want)
	}
}

func TestClientEndpointUsesInternalWhenEnabled(t *testing.T) {
	cfg := OSSConfig{
		Region:      "oss-cn-shanghai",
		Endpoint:    "https://oss-cn-shanghai.aliyuncs.com",
		UseInternal: true,
	}

	got := clientEndpoint(cfg)
	want := "https://oss-cn-shanghai-internal.aliyuncs.com"
	if got != want {
		t.Fatalf("clientEndpoint() = %q, want %q", got, want)
	}
}

func TestClientEndpointKeepsCNameWhenInternalEnabled(t *testing.T) {
	cfg := OSSConfig{
		Region:      "cn-shanghai",
		Endpoint:    "https://warehouse.youlankids.com",
		UseInternal: true,
		UseCName:    true,
	}

	got := clientEndpoint(cfg)
	want := "https://warehouse.youlankids.com"
	if got != want {
		t.Fatalf("clientEndpoint() = %q, want %q", got, want)
	}
}

func TestBrowserDownloadEndpointNeverUsesInternalOSSHost(t *testing.T) {
	tests := []struct {
		name string
		cfg  OSSConfig
	}{
		{
			name: "internal upload enabled",
			cfg: OSSConfig{
				Region:      "oss-cn-shanghai",
				Endpoint:    "https://oss-cn-shanghai.aliyuncs.com",
				UseInternal: true,
			},
		},
		{
			name: "region copied from internal endpoint",
			cfg: OSSConfig{
				Region:      "https://oss-cn-shanghai-internal.aliyuncs.com",
				Endpoint:    "https://oss-cn-shanghai-internal.aliyuncs.com",
				UseInternal: true,
			},
		},
		{
			name: "aliyun internal endpoint is not a cname",
			cfg: OSSConfig{
				Region:   "cn-shanghai",
				Endpoint: "https://oss-cn-shanghai-internal.aliyuncs.com",
				UseCName: true,
			},
		},
		{
			name: "aliyun public endpoint is not a cname",
			cfg: OSSConfig{
				Region:   "cn-shanghai",
				Endpoint: "https://oss-cn-shanghai.aliyuncs.com",
				UseCName: true,
			},
		},
		{
			name: "bucket qualified aliyun endpoint is canonicalized",
			cfg: OSSConfig{
				Region:   "cn-shanghai",
				Endpoint: "https://weather-private.oss-cn-shanghai.aliyuncs.com",
				UseCName: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, useCName := browserDownloadAddressing(tt.cfg)
			want := "https://oss-cn-shanghai.aliyuncs.com"
			if got != want {
				t.Fatalf("browserDownloadEndpoint() = %q, want %q", got, want)
			}
			if useCName {
				t.Fatal("browserDownloadAddressing() kept CNAME mode for a standard Aliyun endpoint")
			}
			if strings.Contains(got, "-internal.") {
				t.Fatalf("browserDownloadEndpoint() returned browser-inaccessible internal endpoint %q", got)
			}
		})
	}
}

func TestBrowserDownloadAddressingPreservesCustomCName(t *testing.T) {
	endpoint, useCName := browserDownloadAddressing(OSSConfig{
		Region:   "cn-shanghai",
		Endpoint: "http://weather-files.example.com",
		UseCName: true,
	})
	if endpoint != "https://weather-files.example.com" || !useCName {
		t.Fatalf("browserDownloadAddressing() = (%q, %t), want custom HTTPS CNAME", endpoint, useCName)
	}
}

func TestBrowserDownloadAddressingSignsAliyunEndpointsWithBucket(t *testing.T) {
	endpoints := []string{
		"https://oss-cn-shanghai-internal.aliyuncs.com",
		"https://oss-cn-shanghai.aliyuncs.com",
		"https://weather-private.oss-cn-shanghai.aliyuncs.com",
	}
	for _, endpointValue := range endpoints {
		t.Run(endpointValue, func(t *testing.T) {
			cfg := OSSConfig{
				Region:   "cn-shanghai",
				Endpoint: endpointValue,
				Bucket:   "weather-private",
				UseCName: true,
			}
			endpoint, useCName := browserDownloadAddressing(cfg)
			sdkClient := alioss.NewClient(alioss.LoadDefaultConfig().
				WithCredentialsProvider(credentials.NewStaticCredentialsProvider("ak", "sk")).
				WithRegion(cfg.Region).
				WithEndpoint(endpoint).
				WithUseCName(useCName).
				WithUseInternalEndpoint(false))
			client := &OSSClient{cfg: cfg, downloadClient: sdkClient}

			signedURL, err := client.PresignDownloadURL(
				t.Context(),
				"mall-weather-exports/job/result.xlsx",
				"mall-weather.xlsx",
				5*time.Minute,
			)
			if err != nil {
				t.Fatalf("PresignDownloadURL() error=%v", err)
			}
			parsedURL, err := url.Parse(signedURL)
			if err != nil {
				t.Fatalf("parse signed URL: %v", err)
			}
			if parsedURL.Scheme != "https" || parsedURL.Hostname() != "weather-private.oss-cn-shanghai.aliyuncs.com" {
				t.Fatalf("signed URL lost bucket addressing for endpoint %q: %q", endpointValue, signedURL)
			}
		})
	}
}

func TestOSSClientUploadPlanMarksMultipart(t *testing.T) {
	client := &OSSClient{
		cfg: OSSConfig{
			Region:                  "cn-shanghai",
			Endpoint:                "https://oss-cn-shanghai.aliyuncs.com",
			MultipartThresholdBytes: 64 * 1024 * 1024,
			PartSizeBytes:           64 * 1024 * 1024,
			ParallelNum:             3,
			EnableCheckpoint:        true,
			CheckpointDir:           "/tmp/oss-checkpoints",
		},
	}

	plan := client.UploadPlan(500 * 1024 * 1024)
	if !plan.Multipart {
		t.Fatal("UploadPlan().Multipart = false, want true for large file")
	}
	if plan.PartSizeBytes != 64*1024*1024 || plan.ParallelNum != 3 || !plan.EnableCheckpoint {
		t.Fatalf("UploadPlan() = %+v, want configured multipart settings", plan)
	}
}

func TestOSSClientPresignDownloadURL(t *testing.T) {
	sdkClient := alioss.NewClient(alioss.LoadDefaultConfig().
		WithCredentialsProvider(credentials.NewStaticCredentialsProvider("ak", "sk")).
		WithRegion("cn-shanghai").
		WithEndpoint("oss-cn-shanghai.aliyuncs.com"))
	client := &OSSClient{cfg: OSSConfig{Bucket: "weather-private"}, client: sdkClient}
	signedURL, err := client.PresignDownloadURL(
		t.Context(),
		"mall-weather-exports/job/result.xlsx",
		"商场天气.xlsx",
		5*time.Minute,
	)
	if err != nil {
		t.Fatalf("PresignDownloadURL() error=%v", err)
	}
	if !strings.Contains(signedURL, "weather-private.oss-cn-shanghai.aliyuncs.com") ||
		!strings.Contains(signedURL, "response-content-disposition") || !strings.Contains(signedURL, "x-oss-signature") {
		t.Fatalf("signed URL=%q", signedURL)
	}
	if _, err := client.PresignDownloadURL(t.Context(), "key", "file.xlsx", 24*time.Hour); err == nil {
		t.Fatal("PresignDownloadURL() accepted excessive expiry")
	}
}

func TestOSSClientPresignDownloadURLUsesBrowserClient(t *testing.T) {
	provider := credentials.NewStaticCredentialsProvider("ak", "sk")
	uploadClient := alioss.NewClient(alioss.LoadDefaultConfig().
		WithCredentialsProvider(provider).
		WithRegion("cn-shanghai").
		WithEndpoint("https://oss-cn-shanghai-internal.aliyuncs.com").
		WithUseInternalEndpoint(true))
	downloadClient := alioss.NewClient(alioss.LoadDefaultConfig().
		WithCredentialsProvider(provider).
		WithRegion("cn-shanghai").
		WithEndpoint("https://oss-cn-shanghai.aliyuncs.com").
		WithUseInternalEndpoint(false))
	client := &OSSClient{
		cfg:            OSSConfig{Bucket: "weather-private", UseInternal: true},
		client:         uploadClient,
		downloadClient: downloadClient,
	}

	signedURL, err := client.PresignDownloadURL(
		t.Context(),
		"mall-weather-exports/job/result.xlsx",
		"商场天气.xlsx",
		5*time.Minute,
	)
	if err != nil {
		t.Fatalf("PresignDownloadURL() error=%v", err)
	}
	if strings.Contains(signedURL, "-internal.") ||
		!strings.Contains(signedURL, "weather-private.oss-cn-shanghai.aliyuncs.com") {
		t.Fatalf("signed URL is not browser-accessible: %q", signedURL)
	}
}
