package weather

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"gin-biz-web-api/pkg/storage"
)

func TestRawSnapshotBuilderBuildsInlineCompressedSnapshot(t *testing.T) {
	builder, err := NewRawSnapshotBuilder(RawSnapshotConfig{
		InlineCompressedBytes: 1024, RetentionDays: 30, SchemaVersion: "caiyun-v26-v1",
	}, nil)
	if err != nil {
		t.Fatalf("NewRawSnapshotBuilder() error=%v", err)
	}
	body := []byte(`{"status":"ok","result":{"realtime":{"temperature":31.2}}}`)
	mallID := uint(7)
	receivedAt := time.Date(2026, 7, 22, 2, 3, 47, 0, time.UTC)
	snapshot, err := builder.Build(context.Background(), RawSnapshotInput{
		Provider: "caiyun", EndpointKind: "v26_weather", MallID: &mallID, ReceivedAtUTC: receivedAt, Body: body,
	})
	if err != nil {
		t.Fatalf("Build() error=%v", err)
	}
	if snapshot.Provider != "caiyun" || snapshot.EndpointKind != "v26_weather" || snapshot.MallID == nil || *snapshot.MallID != 7 ||
		snapshot.Compression != "gzip" || len(snapshot.ContentBlob) == 0 || snapshot.ObjectKey != "" || snapshot.ContentLength != int64(len(body)) ||
		snapshot.SchemaVersion != "caiyun-v26-v1" || snapshot.ExpiresAt == nil || *snapshot.ExpiresAt != receivedAt.Add(30*24*time.Hour) {
		t.Fatalf("snapshot=%+v", snapshot)
	}
	if got := gunzipSnapshot(t, snapshot.ContentBlob); !bytes.Equal(got, body) {
		t.Fatalf("decoded=%q want=%q", got, body)
	}
	body[0] = 'x'
	if got := gunzipSnapshot(t, snapshot.ContentBlob); got[0] != '{' {
		t.Fatalf("snapshot aliases input body: %q", got)
	}
}

func TestRawSnapshotBuilderStoresLargeCompressedSnapshotByObjectKey(t *testing.T) {
	objectStore := &recordingSnapshotObjectStore{}
	builder, err := NewRawSnapshotBuilder(RawSnapshotConfig{
		InlineCompressedBytes: 1, RetentionDays: 60, SchemaVersion: "caiyun-v26-v1",
	}, objectStore)
	if err != nil {
		t.Fatalf("NewRawSnapshotBuilder() error=%v", err)
	}
	body := []byte(`{"status":"ok","result":{}}`)
	snapshot, err := builder.Build(context.Background(), RawSnapshotInput{
		Provider: "caiyun", EndpointKind: "v26_weather", ReceivedAtUTC: time.Date(2026, 7, 22, 2, 3, 47, 0, time.UTC), Body: body,
	})
	if err != nil {
		t.Fatalf("Build() error=%v", err)
	}
	if snapshot.ObjectKey == "" || len(snapshot.ContentBlob) != 0 || objectStore.key == "" || !bytes.Equal(gunzipSnapshot(t, objectStore.body), body) {
		t.Fatalf("snapshot=%+v store=%+v", snapshot, objectStore)
	}
	if strings.Contains(objectStore.key, "secret") || !strings.HasSuffix(objectStore.key, snapshot.ResponseChecksum+".json.gz") {
		t.Fatalf("object key=%q", objectStore.key)
	}
}

func TestRawSnapshotBuilderUsesDefaultsAndEnforcesBodyLimit(t *testing.T) {
	receivedAt := time.Date(2026, 7, 22, 2, 3, 47, 0, time.UTC)
	builder, err := NewRawSnapshotBuilder(RawSnapshotConfig{SchemaVersion: "caiyun-v26-v1"}, nil)
	if err != nil {
		t.Fatalf("NewRawSnapshotBuilder() error=%v", err)
	}
	body := bytes.Repeat([]byte("a"), maximumSnapshotBodyBytes)
	snapshot, err := builder.Build(context.Background(), RawSnapshotInput{
		Provider: "caiyun", EndpointKind: "v26_weather", ReceivedAtUTC: receivedAt, Body: body,
	})
	if err != nil {
		t.Fatalf("Build(maximum body) error=%v", err)
	}
	if snapshot.ContentLength != maximumSnapshotBodyBytes || snapshot.ExpiresAt == nil ||
		*snapshot.ExpiresAt != receivedAt.Add(defaultSnapshotRetentionDays*24*time.Hour) {
		t.Fatalf("snapshot=%+v", snapshot)
	}
	tooLarge := append(body, 'b')
	if _, err := builder.Build(context.Background(), RawSnapshotInput{
		Provider: "caiyun", EndpointKind: "v26_weather", ReceivedAtUTC: receivedAt, Body: tooLarge,
	}); snapshotErrorCode(err) != "invalid_input" {
		t.Fatalf("Build(oversized body) error=%v", err)
	}
}

func TestRawSnapshotBuilderKeepsCompressedBodyAtInlineThreshold(t *testing.T) {
	body := []byte(`{"status":"ok","result":{"realtime":{"temperature":31.2}}}`)
	compressed, err := compressSnapshotForTest(body)
	if err != nil {
		t.Fatalf("compress body: %v", err)
	}
	builder, err := NewRawSnapshotBuilder(RawSnapshotConfig{
		InlineCompressedBytes: len(compressed), SchemaVersion: "caiyun-v26-v1",
	}, nil)
	if err != nil {
		t.Fatalf("NewRawSnapshotBuilder() error=%v", err)
	}
	snapshot, err := builder.Build(context.Background(), RawSnapshotInput{
		Provider: "caiyun", EndpointKind: "v26_weather", ReceivedAtUTC: time.Now().UTC(), Body: body,
	})
	if err != nil {
		t.Fatalf("Build() error=%v", err)
	}
	if len(snapshot.ContentBlob) != len(compressed) || snapshot.ObjectKey != "" {
		t.Fatalf("snapshot=%+v", snapshot)
	}
}

func TestRawSnapshotBuilderReturnsSafeClassifiedErrors(t *testing.T) {
	builder, err := NewRawSnapshotBuilder(RawSnapshotConfig{InlineCompressedBytes: 1, SchemaVersion: "caiyun-v26-v1"}, failingSnapshotObjectStore{})
	if err != nil {
		t.Fatalf("NewRawSnapshotBuilder() error=%v", err)
	}
	_, err = builder.Build(context.Background(), RawSnapshotInput{
		Provider: "caiyun", EndpointKind: "v26_weather", ReceivedAtUTC: time.Now().UTC(), Body: []byte(`{"status":"ok"}`),
	})
	if snapshotErrorCode(err) != "object_store_failed" || strings.Contains(fmt.Sprintf("%+v", err), "secret-object-store-error") {
		t.Fatalf("Build() error=%v", err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = builder.Build(canceled, RawSnapshotInput{
		Provider: "caiyun", EndpointKind: "v26_weather", ReceivedAtUTC: time.Now().UTC(), Body: []byte(`{}`),
	})
	if !errors.Is(err, context.Canceled) || snapshotErrorCode(err) != "canceled" {
		t.Fatalf("canceled error=%v", err)
	}
	if _, err := builder.Build(context.Background(), RawSnapshotInput{}); snapshotErrorCode(err) != "invalid_input" {
		t.Fatalf("invalid input error=%v", err)
	}
}

func TestRawSnapshotBuilderPreservesObjectStoreCancellation(t *testing.T) {
	builder, err := NewRawSnapshotBuilder(RawSnapshotConfig{
		InlineCompressedBytes: 1, SchemaVersion: "caiyun-v26-v1",
	}, canceledSnapshotObjectStore{})
	if err != nil {
		t.Fatalf("NewRawSnapshotBuilder() error=%v", err)
	}
	_, err = builder.Build(context.Background(), RawSnapshotInput{
		Provider: "caiyun", EndpointKind: "v26_weather", ReceivedAtUTC: time.Now().UTC(), Body: []byte(`{"status":"ok"}`),
	})
	if !errors.Is(err, context.Canceled) || snapshotErrorCode(err) != "canceled" {
		t.Fatalf("Build() error=%v", err)
	}
}

func TestOSSSnapshotStoreUsesPrivateTemporaryFileAndCleansIt(t *testing.T) {
	uploader := &recordingFileUploader{}
	store, err := NewOSSSnapshotStore(uploader, OSSSnapshotStoreConfig{
		TempRoot: t.TempDir(), ObjectKeyPrefix: "data-gin",
	})
	if err != nil {
		t.Fatalf("NewOSSSnapshotStore() error=%v", err)
	}
	storedKey, err := store.Put(context.Background(), "weather/raw/test.json.gz", []byte("compressed"))
	if err != nil {
		t.Fatalf("Put() error=%v", err)
	}
	if storedKey != "data-gin/weather/raw/test.json.gz" || !bytes.Equal(uploader.body, []byte("compressed")) || uploader.mode.Perm() != 0o600 {
		t.Fatalf("storedKey=%q uploader=%+v", storedKey, uploader)
	}
	if _, err := os.Stat(uploader.path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary path still exists: %s error=%v", uploader.path, err)
	}
}

func TestOSSSnapshotStoreCleansTemporaryFileWhenUploadFails(t *testing.T) {
	uploader := &recordingFileUploader{uploadErr: errors.New("secret-upload-error")}
	store, err := NewOSSSnapshotStore(uploader, OSSSnapshotStoreConfig{TempRoot: t.TempDir()})
	if err != nil {
		t.Fatalf("NewOSSSnapshotStore() error=%v", err)
	}
	_, err = store.Put(context.Background(), "weather/raw/test.json.gz", []byte("compressed"))
	if err == nil || strings.Contains(err.Error(), "secret-upload-error") {
		t.Fatalf("Put() error=%v", err)
	}
	if _, statErr := os.Stat(uploader.path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("temporary path still exists: %s error=%v", uploader.path, statErr)
	}
}

func TestNewOSSSnapshotStoreRejectsUnsafeObjectKeyPrefix(t *testing.T) {
	uploader := &recordingFileUploader{}
	for _, prefix := range []string{"..", "safe/../escape", `safe\\escape`} {
		t.Run(prefix, func(t *testing.T) {
			_, err := NewOSSSnapshotStore(uploader, OSSSnapshotStoreConfig{
				TempRoot: t.TempDir(), ObjectKeyPrefix: prefix,
			})
			if snapshotErrorCode(err) != "invalid_object_store" {
				t.Fatalf("NewOSSSnapshotStore() error=%v", err)
			}
		})
	}
}

type recordingSnapshotObjectStore struct {
	key  string
	body []byte
}

func (store *recordingSnapshotObjectStore) Put(_ context.Context, objectKey string, compressedBody []byte) (string, error) {
	store.key = objectKey
	store.body = append([]byte(nil), compressedBody...)
	return objectKey, nil
}

type failingSnapshotObjectStore struct{}

func (failingSnapshotObjectStore) Put(context.Context, string, []byte) (string, error) {
	return "", errors.New("secret-object-store-error")
}

type canceledSnapshotObjectStore struct{}

func (canceledSnapshotObjectStore) Put(context.Context, string, []byte) (string, error) {
	return "", &SnapshotError{Code: "canceled", cause: context.Canceled}
}

type recordingFileUploader struct {
	path      string
	body      []byte
	mode      os.FileMode
	uploadErr error
}

func (uploader *recordingFileUploader) UploadFile(_ context.Context, objectKey, localPath, _ string) (storage.UploadResult, error) {
	info, err := os.Stat(localPath)
	if err != nil {
		return storage.UploadResult{}, err
	}
	body, err := os.ReadFile(localPath)
	if err != nil {
		return storage.UploadResult{}, err
	}
	uploader.path = localPath
	uploader.body = body
	uploader.mode = info.Mode()
	if uploader.uploadErr != nil {
		return storage.UploadResult{}, uploader.uploadErr
	}
	return storage.UploadResult{ObjectKey: objectKey}, nil
}

func compressSnapshotForTest(body []byte) ([]byte, error) {
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write(body); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return compressed.Bytes(), nil
}

func gunzipSnapshot(t *testing.T, compressed []byte) []byte {
	t.Helper()
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatalf("gzip.NewReader() error=%v", err)
	}
	decoded, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read gzip snapshot: %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("close gzip snapshot: %v", err)
	}
	return decoded
}
