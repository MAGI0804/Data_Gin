package weather

import (
	"context"
	"errors"
	"os"
	"path"
	"path/filepath"
	"strings"

	"gin-biz-web-api/pkg/storage"
)

type SnapshotFileUploader interface {
	UploadFile(ctx context.Context, objectKey, localPath, downloadName string) (storage.UploadResult, error)
}

type OSSSnapshotStoreConfig struct {
	TempRoot        string
	ObjectKeyPrefix string
}

type OSSSnapshotStore struct {
	uploader        SnapshotFileUploader
	tempRoot        string
	objectKeyPrefix string
}

func NewOSSSnapshotStore(uploader SnapshotFileUploader, config OSSSnapshotStoreConfig) (*OSSSnapshotStore, error) {
	if uploader == nil {
		return nil, &SnapshotError{Code: "invalid_object_store"}
	}
	tempRoot := strings.TrimSpace(config.TempRoot)
	if tempRoot == "" {
		tempRoot = filepath.Join(os.TempDir(), "data-gin-weather-snapshots")
	}
	if !filepath.IsAbs(tempRoot) {
		return nil, &SnapshotError{Code: "invalid_object_store"}
	}
	objectKeyPrefix := strings.Trim(strings.TrimSpace(config.ObjectKeyPrefix), "/")
	if objectKeyPrefix != "" && !validStoredObjectKey(objectKeyPrefix) {
		return nil, &SnapshotError{Code: "invalid_object_store"}
	}
	return &OSSSnapshotStore{
		uploader: uploader, tempRoot: filepath.Clean(tempRoot), objectKeyPrefix: objectKeyPrefix,
	}, nil
}

func (store *OSSSnapshotStore) Put(ctx context.Context, objectKey string, compressedBody []byte) (storedObjectKey string, err error) {
	if store == nil || store.uploader == nil || ctx == nil || !validStoredObjectKey(objectKey) ||
		len(compressedBody) == 0 || len(compressedBody) > maximumCompressedStoreBytes {
		return "", &SnapshotError{Code: "invalid_object_store_input"}
	}
	if err := ctx.Err(); err != nil {
		return "", &SnapshotError{Code: "canceled", cause: err}
	}
	if err := os.MkdirAll(store.tempRoot, 0o700); err != nil {
		return "", &snapshotObjectStoreError{cause: err}
	}
	if err := os.Chmod(store.tempRoot, 0o700); err != nil {
		return "", &snapshotObjectStoreError{cause: err}
	}
	temporary, err := os.CreateTemp(store.tempRoot, "snapshot-*.json.gz")
	if err != nil {
		return "", &snapshotObjectStoreError{cause: err}
	}
	temporaryPath := temporary.Name()
	temporaryClosed := false
	defer func() {
		if !temporaryClosed {
			if closeErr := temporary.Close(); closeErr != nil && err == nil {
				storedObjectKey = ""
				err = &snapshotObjectStoreError{cause: closeErr}
			}
		}
		if removeErr := os.Remove(temporaryPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) && err == nil {
			storedObjectKey = ""
			err = &snapshotObjectStoreError{cause: removeErr}
		}
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return "", &snapshotObjectStoreError{cause: err}
	}
	if _, err := temporary.Write(compressedBody); err != nil {
		return "", &snapshotObjectStoreError{cause: err}
	}
	closeErr := temporary.Close()
	temporaryClosed = true
	if closeErr != nil {
		return "", &snapshotObjectStoreError{cause: closeErr}
	}
	actualObjectKey := path.Join(store.objectKeyPrefix, objectKey)
	if !validStoredObjectKey(actualObjectKey) {
		return "", &SnapshotError{Code: "invalid_object_key"}
	}
	result, err := store.uploader.UploadFile(ctx, actualObjectKey, temporaryPath, filepath.Base(actualObjectKey))
	if err != nil {
		if contextError := ctx.Err(); contextError != nil {
			return "", &SnapshotError{Code: "canceled", cause: contextError}
		}
		return "", &snapshotObjectStoreError{cause: err}
	}
	if !validStoredObjectKey(result.ObjectKey) {
		return "", &SnapshotError{Code: "invalid_object_key"}
	}
	return result.ObjectKey, nil
}

type snapshotObjectStoreError struct {
	cause error
}

func (*snapshotObjectStoreError) Error() string {
	return "weather snapshot: object store operation failed"
}

func (storeError *snapshotObjectStoreError) Unwrap() error {
	if storeError == nil {
		return nil
	}
	return storeError.cause
}
