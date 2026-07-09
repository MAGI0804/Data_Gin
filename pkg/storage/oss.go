package storage

import (
	"context"
	"errors"
	"fmt"
	"mime"
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
}

type OSSClient struct {
	cfg    OSSConfig
	client *alioss.Client
}

type UploadResult struct {
	ObjectKey string
	URL       string
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

	ossCfg := alioss.LoadDefaultConfig().
		WithRegion(cfg.Region).
		WithUseInternalEndpoint(cfg.UseInternal).
		WithUseCName(cfg.UseCName).
		WithDisableSSL(cfg.DisableSSL).
		WithConnectTimeout(time.Duration(cfg.ConnectTimeoutSeconds) * time.Second).
		WithReadWriteTimeout(time.Duration(cfg.ReadWriteTimeoutSeconds) * time.Second).
		WithCredentialsProvider(credentialsProviderFromEnv())
	if cfg.Endpoint != "" {
		ossCfg = ossCfg.WithEndpoint(cfg.Endpoint)
	}

	return &OSSClient{
		cfg:    cfg,
		client: alioss.NewClient(ossCfg),
	}, nil
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
	}
}

func (c *OSSClient) UploadFile(ctx context.Context, objectKey, localPath, downloadName string) (UploadResult, error) {
	objectKey = cleanObjectKey(objectKey)
	if objectKey == "" {
		return UploadResult{}, errors.New("OSS object key 不能为空")
	}
	contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(localPath)))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	request := &alioss.PutObjectRequest{
		Bucket:             alioss.Ptr(c.cfg.Bucket),
		Key:                alioss.Ptr(objectKey),
		ContentType:        alioss.Ptr(contentType),
		CacheControl:       alioss.Ptr("private, max-age=86400"),
		ContentDisposition: alioss.Ptr(contentDisposition(downloadName)),
	}
	if _, err := c.client.PutObjectFromFile(ctx, request, localPath); err != nil {
		return UploadResult{}, err
	}
	return UploadResult{
		ObjectKey: objectKey,
		URL:       c.PublicURL(objectKey),
	}, nil
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
	region = strings.TrimSpace(region)
	region = strings.TrimPrefix(region, "https://")
	region = strings.TrimPrefix(region, "http://")
	region = strings.TrimSuffix(region, ".aliyuncs.com")
	region = strings.TrimPrefix(region, "oss-")
	return strings.Trim(region, "/")
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
