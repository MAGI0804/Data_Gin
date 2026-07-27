package storage

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	alioss "github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"
)

func TestOSSClientStatsDownloadObject(t *testing.T) {
	tests := []struct {
		name       string
		statErr    error
		wantSize   int64
		wantErr    error
		wantAnyErr bool
	}{
		{name: "available", wantSize: 4096},
		{name: "missing", statErr: &alioss.ServiceError{StatusCode: 404, Code: "NoSuchKey"}, wantErr: ErrOSSObjectNotFound},
		{name: "missing alternate code", statErr: &alioss.ServiceError{StatusCode: 404, Code: "ObjectNotExist"}, wantErr: ErrOSSObjectNotFound},
		{name: "bucket missing", statErr: &alioss.ServiceError{StatusCode: 404, Code: "NoSuchBucket"}, wantAnyErr: true},
		{name: "unclassified 404", statErr: &alioss.ServiceError{StatusCode: 404, Code: "BadErrorResponse"}, wantAnyErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var requestedBucket string
			var requestedKey string
			client := &OSSClient{
				cfg: OSSConfig{Bucket: "weather-private"},
				headObject: func(_ context.Context, request *alioss.HeadObjectRequest) (*alioss.HeadObjectResult, error) {
					requestedBucket = alioss.ToString(request.Bucket)
					requestedKey = alioss.ToString(request.Key)
					if tt.statErr != nil {
						return nil, tt.statErr
					}
					return &alioss.HeadObjectResult{ContentLength: 4096}, nil
				},
			}

			metadata, err := client.StatDownloadObject(
				context.Background(),
				"mall-weather-exports/job/result.xlsx",
			)
			wrongError := false
			switch {
			case tt.wantErr != nil:
				wrongError = !errors.Is(err, tt.wantErr)
			case tt.wantAnyErr:
				wrongError = err == nil || errors.Is(err, ErrOSSObjectNotFound)
			default:
				wrongError = err != nil
			}
			if wrongError || metadata.Size != tt.wantSize {
				t.Fatalf("StatDownloadObject() metadata=%+v error=%v, want size=%d error=%v", metadata, err, tt.wantSize, tt.wantErr)
			}
			if requestedBucket != "weather-private" || requestedKey != "mall-weather-exports/job/result.xlsx" {
				t.Fatalf("HeadObject() bucket=%q key=%q", requestedBucket, requestedKey)
			}
		})
	}
}

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

func TestUploadAddressingCanonicalizesAliyunEndpoints(t *testing.T) {
	tests := []struct {
		name string
		cfg  OSSConfig
		want string
	}{
		{
			name: "public service endpoint",
			cfg: OSSConfig{
				Region: "cn-shanghai", Endpoint: "https://oss-cn-shanghai.aliyuncs.com", UseCName: true,
			},
			want: "https://oss-cn-shanghai.aliyuncs.com",
		},
		{
			name: "bucket qualified endpoint",
			cfg: OSSConfig{
				Region: "cn-shanghai", Endpoint: "https://weather-private.oss-cn-shanghai.aliyuncs.com", UseCName: true,
			},
			want: "https://oss-cn-shanghai.aliyuncs.com",
		},
		{
			name: "internal upload",
			cfg: OSSConfig{
				Region: "cn-shanghai", Endpoint: "https://oss-cn-shanghai.aliyuncs.com",
				UseInternal: true, UseCName: true,
			},
			want: "https://oss-cn-shanghai-internal.aliyuncs.com",
		},
		{
			name: "ssl disabled",
			cfg: OSSConfig{
				Region: "cn-shanghai", Endpoint: "https://oss-cn-shanghai.aliyuncs.com",
				UseCName: true, DisableSSL: true,
			},
			want: "http://oss-cn-shanghai.aliyuncs.com",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpoint, useCName := uploadAddressing(tt.cfg)
			if endpoint != tt.want || useCName {
				t.Fatalf("uploadAddressing() = (%q, %t), want (%q, false)", endpoint, useCName, tt.want)
			}
		})
	}
}

func TestUploadAddressingPreservesCustomCName(t *testing.T) {
	endpoint, useCName := uploadAddressing(OSSConfig{
		Region: "cn-shanghai", Endpoint: "https://weather-files.example.com",
		UseInternal: true, UseCName: true,
	})
	if endpoint != "https://weather-files.example.com" || !useCName {
		t.Fatalf("uploadAddressing() = (%q, %t), want custom CNAME", endpoint, useCName)
	}
}

func TestUploadAddressingKeepsBucketInSDKRequests(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		internal bool
		wantHost string
	}{
		{
			name: "public service", endpoint: "https://oss-cn-shanghai.aliyuncs.com",
			wantHost: "weather-private.oss-cn-shanghai.aliyuncs.com",
		},
		{
			name: "bucket qualified service", endpoint: "https://other-bucket.oss-cn-shanghai.aliyuncs.com",
			wantHost: "weather-private.oss-cn-shanghai.aliyuncs.com",
		},
		{
			name: "internal service", endpoint: "https://oss-cn-shanghai-internal.aliyuncs.com",
			internal: true, wantHost: "weather-private.oss-cn-shanghai-internal.aliyuncs.com",
		},
		{
			name: "internal default endpoint", endpoint: "",
			internal: true, wantHost: "weather-private.oss-cn-shanghai-internal.aliyuncs.com",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := OSSConfig{
				Region: "cn-shanghai", Endpoint: tt.endpoint, Bucket: "weather-private",
				UseInternal: tt.internal, UseCName: true,
			}
			endpoint, useCName := uploadAddressing(cfg)
			client := alioss.NewClient(alioss.LoadDefaultConfig().
				WithCredentialsProvider(credentials.NewStaticCredentialsProvider("ak", "sk")).
				WithRegion(cfg.Region).
				WithEndpoint(endpoint).
				WithUseCName(useCName).
				WithUseInternalEndpoint(cfg.UseInternal))
			signed, err := client.Presign(t.Context(), &alioss.PutObjectRequest{
				Bucket: alioss.Ptr(cfg.Bucket), Key: alioss.Ptr("mall-weather-exports/job/result.xlsx"),
			}, alioss.PresignExpires(5*time.Minute))
			if err != nil {
				t.Fatalf("Presign() error=%v", err)
			}
			parsed, err := url.Parse(signed.URL)
			if err != nil || parsed.Hostname() != tt.wantHost {
				t.Fatalf("signed upload host=%q error=%v, want %q", parsed.Hostname(), err, tt.wantHost)
			}
		})
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

func TestOSSClientUploadPlanReportsCanonicalEndpoint(t *testing.T) {
	client := &OSSClient{cfg: OSSConfig{
		Region: "cn-shanghai", Endpoint: "https://oss-cn-shanghai.aliyuncs.com",
		UseInternal: true, UseCName: true,
	}}
	plan := client.UploadPlan(1)
	if plan.Endpoint != "https://oss-cn-shanghai-internal.aliyuncs.com" || !plan.UseInternal {
		t.Fatalf("UploadPlan() = %+v, want canonical internal endpoint", plan)
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

func TestOSSClientStatsInternallyAndPresignsForBrowser(t *testing.T) {
	provider := credentials.NewStaticCredentialsProvider("ak", "sk")
	var statHost string
	uploadHTTPClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		statHost = request.URL.Host
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Length": []string{"4096"}},
			Body:       io.NopCloser(strings.NewReader("")),
			Request:    request,
		}, nil
	})}
	downloadHTTPClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return nil, errors.New("browser download client must not perform object stat")
	})}
	uploadClient := alioss.NewClient(alioss.LoadDefaultConfig().
		WithCredentialsProvider(provider).
		WithRegion("cn-shanghai").
		WithEndpoint("https://oss-cn-shanghai-internal.aliyuncs.com").
		WithUseInternalEndpoint(true).
		WithHttpClient(uploadHTTPClient))
	downloadClient := alioss.NewClient(alioss.LoadDefaultConfig().
		WithCredentialsProvider(provider).
		WithRegion("cn-shanghai").
		WithEndpoint("https://oss-cn-shanghai.aliyuncs.com").
		WithUseInternalEndpoint(false).
		WithHttpClient(downloadHTTPClient))
	client := &OSSClient{
		cfg:            OSSConfig{Bucket: "weather-private", UseInternal: true},
		client:         uploadClient,
		downloadClient: downloadClient,
	}

	metadata, err := client.StatDownloadObject(t.Context(), "mall-weather-exports/job/result.xlsx")
	if err != nil || metadata.Size != 4096 {
		t.Fatalf("StatDownloadObject() metadata=%+v error=%v", metadata, err)
	}
	if statHost != "weather-private.oss-cn-shanghai-internal.aliyuncs.com" {
		t.Fatalf("StatDownloadObject() host=%q, want internal bucket host", statHost)
	}
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
	if err != nil || parsedURL.Hostname() != "weather-private.oss-cn-shanghai.aliyuncs.com" {
		t.Fatalf("PresignDownloadURL() url=%q error=%v, want public bucket host", signedURL, err)
	}
}

func TestOSSClientOpensDownloadObjectThroughConfiguredStorageClient(t *testing.T) {
	var requestedBucket string
	var requestedKey string
	client := &OSSClient{
		cfg: OSSConfig{Bucket: "weather-private"},
		getObject: func(_ context.Context, request *alioss.GetObjectRequest) (*alioss.GetObjectResult, error) {
			requestedBucket = alioss.ToString(request.Bucket)
			requestedKey = alioss.ToString(request.Key)
			return &alioss.GetObjectResult{
				Body:          io.NopCloser(strings.NewReader("PK\x03\x04xlsx")),
				ContentLength: 8,
			}, nil
		},
	}

	object, err := client.OpenDownloadObject(t.Context(), "mall-weather-exports/job/result.xlsx")
	if err != nil {
		t.Fatalf("OpenDownloadObject() error=%v", err)
	}
	defer object.Body.Close()
	body, err := io.ReadAll(object.Body)
	if err != nil || string(body) != "PK\x03\x04xlsx" || object.Size != 8 ||
		requestedBucket != "weather-private" || requestedKey != "mall-weather-exports/job/result.xlsx" {
		t.Fatalf("object=%+v body=%q error=%v bucket=%q key=%q", object, body, err, requestedBucket, requestedKey)
	}
}

func TestOSSClientMapsMissingDownloadObject(t *testing.T) {
	client := &OSSClient{
		cfg: OSSConfig{Bucket: "weather-private"},
		getObject: func(context.Context, *alioss.GetObjectRequest) (*alioss.GetObjectResult, error) {
			return nil, &alioss.ServiceError{StatusCode: http.StatusNotFound, Code: "NoSuchKey"}
		},
	}
	_, err := client.OpenDownloadObject(t.Context(), "mall-weather-exports/job/result.xlsx")
	if !errors.Is(err, ErrOSSObjectNotFound) {
		t.Fatalf("OpenDownloadObject() error=%v, want ErrOSSObjectNotFound", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (roundTrip roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTrip(request)
}
