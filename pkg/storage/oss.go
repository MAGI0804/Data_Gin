package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"gin-biz-web-api/pkg/config"

	alioss "github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"
)

var ErrOSSObjectNotFound = errors.New("oss object not found")

type OSSConfig struct {
	Enabled                 bool
	Region                  string
	Endpoint                string
	Bucket                  string
	CDNBaseURL              string
	Prefix                  string
	UseInternal             bool
	UseCName                bool
	DisableSSL              bool
	ConnectTimeoutSeconds   int
	ReadWriteTimeoutSeconds int
	MultipartThresholdBytes int64
	PartSizeBytes           int64
	ParallelNum             int
	EnableCheckpoint        bool
	CheckpointDir           string
}

type OSSClient struct {
	cfg            OSSConfig
	client         *alioss.Client
	downloadClient *alioss.Client
	headObject     func(context.Context, *alioss.HeadObjectRequest) (*alioss.HeadObjectResult, error)
	getObject      func(context.Context, *alioss.GetObjectRequest) (*alioss.GetObjectResult, error)
}

type UploadResult struct {
	ObjectKey string
	URL       string
}

type ObjectMetadata struct {
	Size int64
}

type DownloadObject struct {
	Body io.ReadCloser
	Size int64
}

type UploadProgress struct {
	Increment   int64
	Transferred int64
	Total       int64
	Percent     float64
}

type UploadProgressFunc func(progress UploadProgress)

type UploadPlan struct {
	Endpoint                string
	UseInternal             bool
	Multipart               bool
	MultipartThresholdBytes int64
	PartSizeBytes           int64
	ParallelNum             int
	EnableCheckpoint        bool
	CheckpointDir           string
}

func OSSStorageEnabled() bool {
	return strings.EqualFold(config.GetString("cfg.storage.driver"), "oss") || config.GetBool("cfg.storage.oss.enabled")
}

func NewOSSClientFromConfig() (*OSSClient, error) {
	cfg := LoadOSSConfig()
	if !cfg.Enabled {
		return nil, errors.New("OSS 存储未启用")
	}
	if cfg.Region == "" {
		return nil, errors.New("OSS region 未配置")
	}
	if cfg.Bucket == "" {
		return nil, errors.New("OSS bucket 未配置")
	}
	if cfg.Endpoint == "" && !cfg.UseInternal {
		return nil, errors.New("OSS endpoint 未配置")
	}

	credentialsProvider := credentialsProviderFromEnv()
	uploadEndpoint, uploadUsesCName := uploadAddressing(cfg)
	ossCfg := alioss.LoadDefaultConfig().
		WithRegion(cfg.Region).
		WithUseInternalEndpoint(cfg.UseInternal).
		WithUseCName(uploadUsesCName).
		WithDisableSSL(cfg.DisableSSL).
		WithConnectTimeout(time.Duration(cfg.ConnectTimeoutSeconds) * time.Second).
		WithReadWriteTimeout(time.Duration(cfg.ReadWriteTimeoutSeconds) * time.Second).
		WithCredentialsProvider(credentialsProvider)
	if uploadEndpoint != "" {
		ossCfg = ossCfg.WithEndpoint(uploadEndpoint)
	}
	downloadEndpoint, downloadUsesCName := browserDownloadAddressing(cfg)
	downloadCfg := alioss.LoadDefaultConfig().
		WithRegion(cfg.Region).
		WithUseInternalEndpoint(false).
		WithUseCName(downloadUsesCName).
		WithDisableSSL(false).
		WithConnectTimeout(time.Duration(cfg.ConnectTimeoutSeconds) * time.Second).
		WithReadWriteTimeout(time.Duration(cfg.ReadWriteTimeoutSeconds) * time.Second).
		WithCredentialsProvider(credentialsProvider)
	if downloadEndpoint != "" {
		downloadCfg = downloadCfg.WithEndpoint(downloadEndpoint)
	}

	client := &OSSClient{
		cfg:            cfg,
		client:         alioss.NewClient(ossCfg),
		downloadClient: alioss.NewClient(downloadCfg),
	}
	client.headObject = func(ctx context.Context, request *alioss.HeadObjectRequest) (*alioss.HeadObjectResult, error) {
		return client.client.HeadObject(ctx, request)
	}
	return client, nil
}

func LoadOSSConfig() OSSConfig {
	connectTimeout := config.GetInt("cfg.storage.oss.connect_timeout")
	if connectTimeout <= 0 {
		connectTimeout = 10
	}
	readWriteTimeout := config.GetInt("cfg.storage.oss.read_write_timeout")
	if readWriteTimeout <= 0 {
		readWriteTimeout = 300
	}
	multipartThreshold := config.GetInt64("cfg.storage.oss.multipart_threshold_bytes")
	if multipartThreshold <= 0 {
		multipartThreshold = 64 * 1024 * 1024
	}
	partSize := config.GetInt64("cfg.storage.oss.part_size_bytes")
	if partSize <= 0 {
		partSize = 64 * 1024 * 1024
	}
	parallelNum := config.GetInt("cfg.storage.oss.parallel_num")
	if parallelNum <= 0 {
		parallelNum = 3
	}
	checkpointDir := strings.TrimSpace(config.GetString("cfg.storage.oss.checkpoint_dir"))
	if checkpointDir == "" {
		checkpointDir = filepath.Join(os.TempDir(), "data-warehouse-oss-checkpoints")
	}
	return OSSConfig{
		Enabled:                 OSSStorageEnabled(),
		Region:                  normalizeOSSRegion(config.GetString("cfg.storage.oss.region")),
		Endpoint:                strings.TrimSpace(config.GetString("cfg.storage.oss.endpoint")),
		Bucket:                  strings.TrimSpace(config.GetString("cfg.storage.oss.bucket")),
		CDNBaseURL:              strings.TrimRight(strings.TrimSpace(config.GetString("cfg.storage.oss.cdn_base_url")), "/"),
		Prefix:                  strings.Trim(strings.TrimSpace(config.GetString("cfg.storage.oss.prefix")), "/"),
		UseInternal:             config.GetBool("cfg.storage.oss.use_internal"),
		UseCName:                config.GetBool("cfg.storage.oss.use_cname"),
		DisableSSL:              config.GetBool("cfg.storage.oss.disable_ssl"),
		ConnectTimeoutSeconds:   connectTimeout,
		ReadWriteTimeoutSeconds: readWriteTimeout,
		MultipartThresholdBytes: multipartThreshold,
		PartSizeBytes:           partSize,
		ParallelNum:             parallelNum,
		EnableCheckpoint:        config.GetBool("cfg.storage.oss.enable_checkpoint", true),
		CheckpointDir:           checkpointDir,
	}
}

func (c *OSSClient) UploadFile(ctx context.Context, objectKey, localPath, downloadName string) (UploadResult, error) {
	return c.UploadFileWithProgress(ctx, objectKey, localPath, downloadName, nil)
}

func (c *OSSClient) UploadFileWithProgress(ctx context.Context, objectKey, localPath, downloadName string, onProgress UploadProgressFunc) (UploadResult, error) {
	objectKey = cleanObjectKey(objectKey)
	if objectKey == "" {
		return UploadResult{}, errors.New("OSS object key cannot be empty")
	}
	fileInfo, err := os.Stat(localPath)
	if err != nil {
		return UploadResult{}, err
	}
	contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(localPath)))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	request := &alioss.PutObjectRequest{
		Bucket:             alioss.Ptr(c.cfg.Bucket),
		Key:                alioss.Ptr(objectKey),
		ContentLength:      alioss.Ptr(fileInfo.Size()),
		ContentType:        alioss.Ptr(contentType),
		CacheControl:       alioss.Ptr("private, max-age=86400"),
		ContentDisposition: alioss.Ptr(contentDisposition(downloadName)),
	}
	if onProgress != nil {
		request.ProgressFn = func(increment, transferred, total int64) {
			percent := float64(0)
			if total > 0 {
				percent = float64(transferred) * 100 / float64(total)
			}
			onProgress(UploadProgress{
				Increment:   increment,
				Transferred: transferred,
				Total:       total,
				Percent:     percent,
			})
		}
	}
	if fileInfo.Size() >= c.cfg.MultipartThresholdBytes {
		if c.cfg.EnableCheckpoint {
			if err := os.MkdirAll(c.cfg.CheckpointDir, 0700); err != nil {
				return UploadResult{}, err
			}
		}
		uploader := c.client.NewUploader(func(options *alioss.UploaderOptions) {
			options.PartSize = c.cfg.PartSizeBytes
			options.ParallelNum = c.cfg.ParallelNum
			options.EnableCheckpoint = c.cfg.EnableCheckpoint
			options.CheckpointDir = c.cfg.CheckpointDir
		})
		if _, err := uploader.UploadFile(ctx, request, localPath); err != nil {
			return UploadResult{}, err
		}
	} else {
		if _, err := c.client.PutObjectFromFile(ctx, request, localPath); err != nil {
			return UploadResult{}, err
		}
	}
	return UploadResult{
		ObjectKey: objectKey,
		URL:       c.PublicURL(objectKey),
	}, nil
}

func (c *OSSClient) UploadPlan(fileSize int64) UploadPlan {
	endpoint, _ := uploadAddressing(c.cfg)
	return UploadPlan{
		Endpoint:                endpoint,
		UseInternal:             c.cfg.UseInternal,
		Multipart:               fileSize >= c.cfg.MultipartThresholdBytes,
		MultipartThresholdBytes: c.cfg.MultipartThresholdBytes,
		PartSizeBytes:           c.cfg.PartSizeBytes,
		ParallelNum:             c.cfg.ParallelNum,
		EnableCheckpoint:        c.cfg.EnableCheckpoint,
		CheckpointDir:           c.cfg.CheckpointDir,
	}
}

func (c *OSSClient) DeleteObject(ctx context.Context, objectKey string) error {
	objectKey = cleanObjectKey(objectKey)
	if objectKey == "" {
		return nil
	}
	_, err := c.client.DeleteObject(ctx, &alioss.DeleteObjectRequest{
		Bucket: alioss.Ptr(c.cfg.Bucket),
		Key:    alioss.Ptr(objectKey),
	})
	return err
}

func (c *OSSClient) PresignDownloadURL(
	ctx context.Context,
	objectKey string,
	downloadName string,
	expires time.Duration,
) (string, error) {
	objectKey = cleanObjectKey(objectKey)
	if c == nil || ctx == nil || objectKey == "" || expires < time.Minute || expires > time.Hour {
		return "", fmt.Errorf("OSS 下载签名参数无效")
	}
	presignClient := c.downloadClient
	if presignClient == nil {
		presignClient = c.client
	}
	if presignClient == nil {
		return "", fmt.Errorf("OSS 下载签名客户端无效")
	}
	result, err := presignClient.Presign(ctx, &alioss.GetObjectRequest{
		Bucket:                     alioss.Ptr(c.cfg.Bucket),
		Key:                        alioss.Ptr(objectKey),
		ResponseCacheControl:       alioss.Ptr("private, no-store"),
		ResponseContentDisposition: alioss.Ptr(contentDisposition(downloadName)),
	}, alioss.PresignExpires(expires))
	if err != nil {
		return "", fmt.Errorf("OSS 生成下载签名失败: %w", err)
	}
	if result == nil || strings.TrimSpace(result.URL) == "" {
		return "", fmt.Errorf("OSS 下载签名结果无效")
	}
	return result.URL, nil
}

func (c *OSSClient) StatDownloadObject(ctx context.Context, objectKey string) (ObjectMetadata, error) {
	objectKey = cleanObjectKey(objectKey)
	if c == nil || ctx == nil || objectKey == "" {
		return ObjectMetadata{}, fmt.Errorf("OSS 下载对象查询参数无效")
	}
	headObject := c.headObject
	if headObject == nil && c.client != nil {
		headObject = func(ctx context.Context, request *alioss.HeadObjectRequest) (*alioss.HeadObjectResult, error) {
			return c.client.HeadObject(ctx, request)
		}
	}
	if headObject == nil && c.downloadClient != nil {
		headObject = func(ctx context.Context, request *alioss.HeadObjectRequest) (*alioss.HeadObjectResult, error) {
			return c.downloadClient.HeadObject(ctx, request)
		}
	}
	if headObject == nil {
		return ObjectMetadata{}, fmt.Errorf("OSS 下载对象查询客户端无效")
	}
	result, err := headObject(ctx, &alioss.HeadObjectRequest{
		Bucket: alioss.Ptr(c.cfg.Bucket),
		Key:    alioss.Ptr(objectKey),
	})
	if err != nil {
		var serviceError *alioss.ServiceError
		if errors.As(err, &serviceError) && ossObjectMissing(serviceError) {
			return ObjectMetadata{}, fmt.Errorf("%w: %w", ErrOSSObjectNotFound, err)
		}
		return ObjectMetadata{}, fmt.Errorf("OSS 查询下载对象失败: %w", err)
	}
	if result == nil || result.ContentLength < 0 {
		return ObjectMetadata{}, fmt.Errorf("OSS 下载对象元数据无效")
	}
	return ObjectMetadata{Size: result.ContentLength}, nil
}

func (c *OSSClient) OpenDownloadObject(ctx context.Context, objectKey string) (DownloadObject, error) {
	objectKey = cleanObjectKey(objectKey)
	if c == nil || ctx == nil || objectKey == "" {
		return DownloadObject{}, fmt.Errorf("OSS 下载对象读取参数无效")
	}
	getObject := c.getObject
	if getObject == nil && c.downloadClient != nil {
		getObject = func(ctx context.Context, request *alioss.GetObjectRequest) (*alioss.GetObjectResult, error) {
			return c.downloadClient.GetObject(ctx, request)
		}
	}
	if getObject == nil && c.client != nil {
		getObject = func(ctx context.Context, request *alioss.GetObjectRequest) (*alioss.GetObjectResult, error) {
			return c.client.GetObject(ctx, request)
		}
	}
	if getObject == nil {
		return DownloadObject{}, fmt.Errorf("OSS 下载对象读取客户端无效")
	}
	result, err := getObject(ctx, &alioss.GetObjectRequest{
		Bucket: alioss.Ptr(c.cfg.Bucket),
		Key:    alioss.Ptr(objectKey),
	})
	if err != nil {
		var serviceError *alioss.ServiceError
		if errors.As(err, &serviceError) && ossObjectMissing(serviceError) {
			return DownloadObject{}, fmt.Errorf("%w: %w", ErrOSSObjectNotFound, err)
		}
		return DownloadObject{}, fmt.Errorf("OSS 读取下载对象失败: %w", err)
	}
	if result == nil || result.Body == nil || result.ContentLength < 0 {
		if result != nil && result.Body != nil {
			_ = result.Body.Close()
		}
		return DownloadObject{}, fmt.Errorf("OSS 下载对象响应无效")
	}
	return DownloadObject{
		Body: result.Body,
		Size: result.ContentLength,
	}, nil
}

func ossObjectMissing(serviceError *alioss.ServiceError) bool {
	if serviceError == nil || serviceError.StatusCode != http.StatusNotFound {
		return false
	}
	switch serviceError.Code {
	case "NoSuchKey", "ObjectNotExist":
		return true
	default:
		return false
	}
}

func (c *OSSClient) PublicURL(objectKey string) string {
	objectKey = cleanObjectKey(objectKey)
	if c.cfg.CDNBaseURL != "" {
		return c.cfg.CDNBaseURL + "/" + strings.TrimLeft(objectKey, "/")
	}
	scheme := "https"
	if c.cfg.DisableSSL {
		scheme = "http"
	}
	host := strings.TrimPrefix(strings.TrimPrefix(c.cfg.Endpoint, "https://"), "http://")
	if c.cfg.UseCName && host != "" {
		return fmt.Sprintf("%s://%s/%s", scheme, strings.TrimRight(host, "/"), objectKey)
	}
	if host == "" {
		host = fmt.Sprintf("oss-%s.aliyuncs.com", normalizeOSSRegion(c.cfg.Region))
	}
	return fmt.Sprintf("%s://%s.%s/%s", scheme, c.cfg.Bucket, strings.TrimRight(host, "/"), objectKey)
}

func normalizeOSSRegion(region string) string {
	region = strings.ToLower(strings.TrimSpace(region))
	region = strings.TrimPrefix(region, "https://")
	region = strings.TrimPrefix(region, "http://")
	region = strings.SplitN(region, "/", 2)[0]
	region = strings.TrimSuffix(region, ".aliyuncs.com")
	region = strings.TrimSuffix(region, "-internal")
	if marker := strings.LastIndex(region, ".oss-"); marker >= 0 {
		region = region[marker+1:]
	}
	region = strings.TrimPrefix(region, "oss-")
	return strings.Trim(region, "/")
}

func clientEndpoint(cfg OSSConfig) string {
	endpoint := strings.TrimSpace(cfg.Endpoint)
	if cfg.UseInternal && !cfg.UseCName {
		return internalEndpoint(cfg.Region, cfg.DisableSSL)
	}
	return endpoint
}

func uploadAddressing(cfg OSSConfig) (string, bool) {
	endpoint := strings.TrimSpace(cfg.Endpoint)
	if endpoint == "" {
		if cfg.UseInternal {
			return internalEndpoint(cfg.Region, cfg.DisableSSL), false
		}
		return "", false
	}
	if !isAliyunOSSEndpoint(endpoint) {
		return clientEndpoint(cfg), cfg.UseCName
	}
	region := normalizeOSSRegion(cfg.Region)
	if region == "" {
		region = normalizeOSSRegion(endpoint)
	}
	if region == "" {
		return "", false
	}
	if cfg.UseInternal {
		return internalEndpoint(region, cfg.DisableSSL), false
	}
	scheme := "https"
	if cfg.DisableSSL {
		scheme = "http"
	}
	return fmt.Sprintf("%s://oss-%s.aliyuncs.com", scheme, region), false
}

func browserDownloadAddressing(cfg OSSConfig) (string, bool) {
	endpoint := strings.TrimSpace(cfg.Endpoint)
	if isAliyunOSSEndpoint(endpoint) {
		region := normalizeOSSRegion(cfg.Region)
		if region == "" {
			region = normalizeOSSRegion(endpoint)
		}
		if region == "" {
			return "", false
		}
		return "https://oss-" + region + ".aliyuncs.com", false
	}
	if cfg.UseCName && endpoint != "" {
		return httpsEndpoint(endpoint), true
	}
	if cfg.UseInternal || endpoint == "" || strings.Contains(endpoint, "-internal.") {
		region := normalizeOSSRegion(cfg.Region)
		if region == "" {
			return "", false
		}
		return "https://oss-" + region + ".aliyuncs.com", false
	}
	return httpsEndpoint(endpoint), false
}

func isAliyunOSSEndpoint(endpoint string) bool {
	host := strings.ToLower(endpointHostname(endpoint))
	if !strings.HasSuffix(host, ".aliyuncs.com") {
		return false
	}
	return strings.HasPrefix(host, "oss-") || strings.Contains(host, ".oss-")
}

func endpointHostname(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return ""
	}
	if !strings.Contains(endpoint, "://") {
		endpoint = "https://" + endpoint
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return ""
	}
	return parsed.Hostname()
}

func httpsEndpoint(endpoint string) string {
	host := strings.TrimSpace(endpoint)
	host = strings.TrimPrefix(host, "https://")
	host = strings.TrimPrefix(host, "http://")
	host = strings.TrimRight(host, "/")
	if host == "" {
		return ""
	}
	return "https://" + host
}

func internalEndpoint(region string, disableSSL bool) string {
	region = normalizeOSSRegion(region)
	if region == "" {
		return ""
	}
	scheme := "https"
	if disableSSL {
		scheme = "http"
	}
	return fmt.Sprintf("%s://oss-%s-internal.aliyuncs.com", scheme, region)
}

func BuildObjectKey(parts ...string) string {
	cfg := LoadOSSConfig()
	allParts := make([]string, 0, len(parts)+1)
	if cfg.Prefix != "" {
		allParts = append(allParts, cfg.Prefix)
	}
	allParts = append(allParts, parts...)
	return cleanObjectKey(path.Join(allParts...))
}

func credentialsProviderFromEnv() credentials.CredentialsProvider {
	id := firstEnv("ALIYUN_OSS_ACCESS_KEY_ID", "OSS_ACCESS_KEY_ID", "ALIBABA_CLOUD_ACCESS_KEY_ID")
	secret := firstEnv("ALIYUN_OSS_ACCESS_KEY_SECRET", "OSS_ACCESS_KEY_SECRET", "ALIBABA_CLOUD_ACCESS_KEY_SECRET")
	token := firstEnv("ALIYUN_OSS_SECURITY_TOKEN", "OSS_SECURITY_TOKEN", "ALIBABA_CLOUD_SECURITY_TOKEN")
	if id != "" && secret != "" {
		if token != "" {
			return credentials.NewStaticCredentialsProvider(id, secret, token)
		}
		return credentials.NewStaticCredentialsProvider(id, secret)
	}
	return credentials.NewEnvironmentVariableCredentialsProvider()
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

func cleanObjectKey(objectKey string) string {
	objectKey = strings.ReplaceAll(objectKey, "\\", "/")
	objectKey = path.Clean("/" + objectKey)
	objectKey = strings.TrimLeft(objectKey, "/")
	if objectKey == "." {
		return ""
	}
	return objectKey
}

func contentDisposition(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	if name == "" || name == "." {
		name = "download"
	}
	escaped := url.PathEscape(name)
	return fmt.Sprintf("attachment; filename=%q; filename*=UTF-8''%s", name, escaped)
}
