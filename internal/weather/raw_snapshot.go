package weather

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/compressutil"
)

const (
	defaultSnapshotInlineBytes   = 4 << 20
	maximumSnapshotBodyBytes     = 16 << 20
	maximumCompressedStoreBytes  = 32 << 20
	defaultSnapshotRetentionDays = 60
)

var snapshotNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,31}$`)

type SnapshotObjectStore interface {
	Put(ctx context.Context, objectKey string, compressedBody []byte) (storedObjectKey string, err error)
}

type RawSnapshotConfig struct {
	InlineCompressedBytes int
	RetentionDays         int
	SchemaVersion         string
}

type RawSnapshotInput struct {
	Provider      string
	EndpointKind  string
	MallID        *uint
	ReceivedAtUTC time.Time
	Body          []byte
}

type SnapshotError struct {
	Code  string
	cause error
}

func (snapshotError *SnapshotError) Error() string {
	if snapshotError == nil || snapshotError.Code == "" {
		return "weather snapshot: operation failed"
	}
	return "weather snapshot: " + snapshotError.Code
}

func (snapshotError *SnapshotError) Unwrap() error {
	if snapshotError == nil {
		return nil
	}
	return snapshotError.cause
}

type RawSnapshotBuilder struct {
	inlineCompressedBytes int
	retentionDays         int
	schemaVersion         string
	objectStore           SnapshotObjectStore
}

func NewRawSnapshotBuilder(config RawSnapshotConfig, objectStore SnapshotObjectStore) (*RawSnapshotBuilder, error) {
	if config.InlineCompressedBytes == 0 {
		config.InlineCompressedBytes = defaultSnapshotInlineBytes
	}
	if config.RetentionDays == 0 {
		config.RetentionDays = defaultSnapshotRetentionDays
	}
	config.SchemaVersion = strings.TrimSpace(config.SchemaVersion)
	if config.InlineCompressedBytes < 0 || config.InlineCompressedBytes > maximumCompressedStoreBytes ||
		config.RetentionDays < 1 || config.RetentionDays > 3650 || !snapshotNamePattern.MatchString(config.SchemaVersion) {
		return nil, &SnapshotError{Code: "invalid_config"}
	}
	return &RawSnapshotBuilder{
		inlineCompressedBytes: config.InlineCompressedBytes,
		retentionDays:         config.RetentionDays,
		schemaVersion:         config.SchemaVersion,
		objectStore:           objectStore,
	}, nil
}

func (builder *RawSnapshotBuilder) Build(ctx context.Context, input RawSnapshotInput) (*model.ProviderRawSnapshot, error) {
	if builder == nil || ctx == nil || !snapshotNamePattern.MatchString(input.Provider) || !snapshotNamePattern.MatchString(input.EndpointKind) ||
		!validMappingTime(input.ReceivedAtUTC) || len(input.Body) == 0 || len(input.Body) > maximumSnapshotBodyBytes ||
		(input.MallID != nil && *input.MallID == 0) {
		return nil, &SnapshotError{Code: "invalid_input"}
	}
	if err := ctx.Err(); err != nil {
		return nil, &SnapshotError{Code: "canceled", cause: err}
	}
	compressed, err := compressutil.Gzip(input.Body)
	if err != nil || len(compressed) > maximumCompressedStoreBytes {
		return nil, &SnapshotError{Code: "compression_failed", cause: err}
	}
	checksumBytes := sha256.Sum256(input.Body)
	checksum := hex.EncodeToString(checksumBytes[:])
	receivedAtUTC := input.ReceivedAtUTC.UTC()
	expiresAt := receivedAtUTC.Add(time.Duration(builder.retentionDays) * 24 * time.Hour)
	snapshot := &model.ProviderRawSnapshot{
		Provider: input.Provider, EndpointKind: input.EndpointKind,
		ResponseChecksum: checksum, Compression: "gzip", ContentLength: int64(len(input.Body)),
		SchemaVersion: builder.schemaVersion, ExpiresAt: &expiresAt,
	}
	if input.MallID != nil {
		mallID := *input.MallID
		snapshot.MallID = &mallID
	}
	if len(compressed) <= builder.inlineCompressedBytes {
		snapshot.ContentBlob = append([]byte(nil), compressed...)
		return snapshot, nil
	}
	if builder.objectStore == nil {
		return nil, &SnapshotError{Code: "object_store_unavailable"}
	}
	if err := ctx.Err(); err != nil {
		return nil, &SnapshotError{Code: "canceled", cause: err}
	}
	storedObjectKey, err := builder.objectStore.Put(ctx, snapshotObjectKey(input, checksum), compressed)
	if err != nil {
		var classifiedError *SnapshotError
		if errors.As(err, &classifiedError) {
			return nil, classifiedError
		}
		return nil, &SnapshotError{Code: "object_store_failed", cause: err}
	}
	if !validStoredObjectKey(storedObjectKey) {
		return nil, &SnapshotError{Code: "invalid_object_key"}
	}
	snapshot.ObjectKey = storedObjectKey
	return snapshot, nil
}

func snapshotObjectKey(input RawSnapshotInput, checksum string) string {
	receivedAtUTC := input.ReceivedAtUTC.UTC()
	owner := "global"
	if input.MallID != nil {
		owner = "mall-" + strconv.FormatUint(uint64(*input.MallID), 10)
	}
	return path.Join(
		"weather", "raw", input.Provider, input.EndpointKind,
		receivedAtUTC.Format("2006"), receivedAtUTC.Format("01"), receivedAtUTC.Format("02"),
		owner, checksum+".json.gz",
	)
}

func validStoredObjectKey(objectKey string) bool {
	if objectKey == "" || len(objectKey) > 1024 || strings.TrimSpace(objectKey) != objectKey || strings.HasPrefix(objectKey, "/") ||
		strings.Contains(objectKey, "\\") || path.Clean(objectKey) != objectKey {
		return false
	}
	for _, segment := range strings.Split(objectKey, "/") {
		if segment == "." || segment == ".." {
			return false
		}
	}
	for _, character := range objectKey {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func snapshotErrorCode(err error) string {
	var snapshotError *SnapshotError
	if errors.As(err, &snapshotError) {
		return snapshotError.Code
	}
	return ""
}

func (builder *RawSnapshotBuilder) String() string {
	if builder == nil {
		return "weather.RawSnapshotBuilder{nil}"
	}
	return fmt.Sprintf("weather.RawSnapshotBuilder{inline_bytes:%d,retention_days:%d,schema_version:%q}",
		builder.inlineCompressedBytes, builder.retentionDays, builder.schemaVersion)
}
