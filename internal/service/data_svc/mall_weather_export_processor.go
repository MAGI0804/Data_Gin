package data_svc

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/model"

	"github.com/google/uuid"
)

const mallWeatherExportWorkRootName = "mall-weather-exports"

type mallWeatherExportPreparedJob struct {
	Config   MallWeatherExportProfileConfig
	Filter   data_dao.MallWeatherExportEstimateFilter
	FileName string
}

func prepareMallWeatherExportJob(
	row model.MallWeatherExportJob,
	now time.Time,
) (mallWeatherExportPreparedJob, error) {
	if row.ID == 0 || row.ProfileID == 0 || row.ProfileVersion == 0 ||
		len(row.JobUUID) != 36 || uuid.Validate(row.JobUUID) != nil || row.TotalRows < 0 ||
		row.TotalRows > maxMallWeatherExportConfiguredRows || row.CreatedAt.IsZero() || now.IsZero() {
		return mallWeatherExportPreparedJob{}, fmt.Errorf("mall weather export processor: invalid job identity")
	}
	var snapshot MallWeatherExportProfileSnapshot
	if err := decodeMallWeatherExportStoredJSON(row.ProfileSnapshotJSON, &snapshot); err != nil {
		return mallWeatherExportPreparedJob{}, fmt.Errorf("mall weather export processor: decode profile snapshot: %w", err)
	}
	if snapshot.ProfileID != row.ProfileID || snapshot.Version != row.ProfileVersion {
		return mallWeatherExportPreparedJob{}, fmt.Errorf("mall weather export processor: profile snapshot identity mismatch")
	}
	originalConfig, err := json.Marshal(snapshot.Config)
	if err != nil {
		return mallWeatherExportPreparedJob{}, fmt.Errorf("mall weather export processor: encode profile snapshot: %w", err)
	}
	normalizedProfile, normalizedConfig, err := normalizeMallWeatherExportProfile(requestbody.MallWeatherExportProfileSaveRequest{
		Code:             snapshot.Code,
		Name:             snapshot.Name,
		TimeZone:         snapshot.Config.TimeZone,
		UnitSystem:       snapshot.Config.UnitSystem,
		DateFormat:       snapshot.Config.DateFormat,
		DateTimeFormat:   snapshot.Config.DateTimeFormat,
		FileNameTemplate: snapshot.Config.FileNameTemplate,
		Filters:          snapshot.Config.Filters,
		Datasets:         snapshot.Config.Datasets,
	})
	normalizedConfigJSON, encodeErr := json.Marshal(normalizedConfig)
	identityChanged := normalizedProfile.Code != snapshot.Code || normalizedProfile.Name != snapshot.Name
	if err != nil || encodeErr != nil || identityChanged || !bytes.Equal(originalConfig, normalizedConfigJSON) {
		return mallWeatherExportPreparedJob{}, fmt.Errorf("mall weather export processor: invalid normalized profile snapshot")
	}
	var filters requestbody.MallWeatherExportFilters
	if err := decodeMallWeatherExportStoredJSON(row.FiltersJSON, &filters); err != nil {
		return mallWeatherExportPreparedJob{}, fmt.Errorf("mall weather export processor: decode filters: %w", err)
	}
	originalFilters, err := json.Marshal(filters)
	if err != nil {
		return mallWeatherExportPreparedJob{}, fmt.Errorf("mall weather export processor: encode filters: %w", err)
	}
	filters, err = normalizeMallWeatherExportFilters(filters)
	normalizedFilters, encodeErr := json.Marshal(filters)
	if err != nil || encodeErr != nil || !bytes.Equal(originalFilters, normalizedFilters) {
		return mallWeatherExportPreparedJob{}, fmt.Errorf("mall weather export processor: invalid normalized filters")
	}
	if err := validateMallWeatherExportJobRange(
		normalizedConfig.Datasets,
		filters,
		normalizedConfig.TimeZone,
		mallWeatherExportLimits{MaxRangeDays: maxMallWeatherExportConfiguredRangeDays},
	); err != nil {
		return mallWeatherExportPreparedJob{}, fmt.Errorf("mall weather export processor: invalid stored range: %w", err)
	}
	estimateRequest, err := mallWeatherExportEstimateRequest(MallWeatherExportProfileDTO{
		TimeZone: normalizedConfig.TimeZone,
		Datasets: normalizedConfig.Datasets,
	}, filters, 1)
	if err != nil {
		return mallWeatherExportPreparedJob{}, fmt.Errorf("mall weather export processor: build data filter: %w", err)
	}
	location, err := time.LoadLocation(normalizedConfig.TimeZone)
	if err != nil {
		return mallWeatherExportPreparedJob{}, fmt.Errorf("mall weather export processor: load time zone: %w", err)
	}
	fileName, err := renderMallWeatherExportFileName(normalizedConfig.FileNameTemplate, now, location)
	if err != nil {
		return mallWeatherExportPreparedJob{}, err
	}
	return mallWeatherExportPreparedJob{
		Config:   normalizedConfig,
		Filter:   estimateRequest.Filter,
		FileName: fileName,
	}, nil
}

func decodeMallWeatherExportStoredJSON(value model.JSONText, destination interface{}) error {
	if len(value) == 0 || destination == nil {
		return fmt.Errorf("mall weather export processor: empty stored JSON")
	}
	decoder := json.NewDecoder(strings.NewReader(string(value)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("mall weather export processor: decode stored JSON: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return fmt.Errorf("mall weather export processor: stored JSON has trailing data")
	}
	return nil
}

func createMallWeatherExportWorkDir(root string, jobID uint, runToken string) (string, error) {
	root = filepath.Clean(strings.TrimSpace(root))
	if root == "." || jobID == 0 || len(runToken) != 36 || uuid.Validate(runToken) != nil {
		return "", fmt.Errorf("mall weather export processor: invalid work directory identity")
	}
	workDir := filepath.Join(
		root,
		mallWeatherExportWorkRootName,
		strconv.FormatUint(uint64(jobID), 10),
		runToken,
	)
	if !isPathInside(root, workDir) {
		return "", fmt.Errorf("mall weather export processor: unsafe work directory")
	}
	if err := os.MkdirAll(workDir, 0o700); err != nil {
		return "", fmt.Errorf("mall weather export processor: create work directory: %w", err)
	}
	info, err := os.Lstat(workDir)
	if err != nil {
		return "", fmt.Errorf("mall weather export processor: inspect work directory: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("mall weather export processor: unsafe work directory type")
	}
	if err := os.Chmod(workDir, 0o700); err != nil {
		return "", fmt.Errorf("mall weather export processor: secure work directory: %w", err)
	}
	return workDir, nil
}

func inspectMallWeatherExportArtifact(filePath string) (string, int64, error) {
	info, err := os.Lstat(filePath)
	if err != nil {
		return "", 0, fmt.Errorf("mall weather export processor: inspect artifact: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() <= 0 {
		return "", 0, fmt.Errorf("mall weather export processor: invalid artifact")
	}
	if err := os.Chmod(filePath, 0o600); err != nil {
		return "", 0, fmt.Errorf("mall weather export processor: secure artifact: %w", err)
	}
	file, err := os.Open(filePath)
	if err != nil {
		return "", 0, fmt.Errorf("mall weather export processor: open artifact: %w", err)
	}
	hash := sha256.New()
	_, copyErr := io.Copy(hash, file)
	closeErr := file.Close()
	if copyErr != nil || closeErr != nil {
		var artifactErr error
		if copyErr != nil {
			artifactErr = errors.Join(artifactErr, fmt.Errorf("mall weather export processor: hash artifact: %w", copyErr))
		}
		if closeErr != nil {
			artifactErr = errors.Join(artifactErr, fmt.Errorf("mall weather export processor: close artifact: %w", closeErr))
		}
		return "", 0, artifactErr
	}
	return hex.EncodeToString(hash.Sum(nil)), info.Size(), nil
}

func mallWeatherExportObjectKey(jobUUID, runToken string, now time.Time) (string, error) {
	if len(jobUUID) != 36 || uuid.Validate(jobUUID) != nil ||
		len(runToken) != 36 || uuid.Validate(runToken) != nil || now.IsZero() {
		return "", fmt.Errorf("mall weather export processor: invalid object identity")
	}
	return path.Join(
		mallWeatherExportWorkRootName,
		now.UTC().Format("2006/01/02"),
		jobUUID,
		runToken,
		"result.xlsx",
	), nil
}
