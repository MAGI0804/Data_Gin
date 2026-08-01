package data_ctrl

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const monitoringMaxPageSize = 100

var monitoringQueryLocation = time.FixedZone("Asia/Shanghai", 8*60*60)

type monitoringPagination struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

func monitoringQueryKeysAllowed(values url.Values, allowed ...string) bool {
	permitted := make(map[string]struct{}, len(allowed))
	for _, key := range allowed {
		permitted[key] = struct{}{}
	}
	for key := range values {
		if _, ok := permitted[key]; !ok {
			return false
		}
	}
	return true
}

func parseMonitoringPagination(values url.Values) (page, pageSize int, err error) {
	page, err = parseMonitoringPositiveInt(values.Get("page"), 1, 1_000_000)
	if err != nil {
		return 0, 0, err
	}
	pageSize, err = parseMonitoringPositiveInt(values.Get("page_size"), 20, monitoringMaxPageSize)
	if err != nil {
		return 0, 0, err
	}
	return page, pageSize, nil
}

func parseMonitoringPositiveInt(value string, fallback, maximum int) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 || parsed > maximum {
		return 0, fmt.Errorf("invalid positive integer")
	}
	return parsed, nil
}

func parseMonitoringTime(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return &parsed, nil
	}
	parsed, err := time.ParseInLocation("2006-01-02T15:04", value, monitoringQueryLocation)
	if err != nil {
		return nil, fmt.Errorf("invalid monitoring time")
	}
	return &parsed, nil
}

func monitoringTimeRangeValid(start, end *time.Time) bool {
	return start == nil || end == nil || !end.Before(*start)
}

func monitoringPaginationResponse(page, pageSize int, total int64) monitoringPagination {
	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))
	return monitoringPagination{Page: page, PageSize: pageSize, Total: total, TotalPages: totalPages}
}

func parseMonitoringBool(value string) (*bool, error) {
	switch strings.TrimSpace(value) {
	case "":
		return nil, nil
	case "true":
		result := true
		return &result, nil
	case "false":
		result := false
		return &result, nil
	default:
		return nil, fmt.Errorf("invalid boolean")
	}
}

func parseMonitoringUint(value string) (uint, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseUint(value, 10, 32)
	if err != nil || parsed == 0 {
		return 0, fmt.Errorf("invalid unsigned integer")
	}
	return uint(parsed), nil
}

func parseMonitoringText(value string, maximum int) (string, error) {
	value = strings.TrimSpace(value)
	if len(value) > maximum {
		return "", fmt.Errorf("invalid text")
	}
	return value, nil
}

func monitoringHasAnyKey(values url.Values, keys ...string) bool {
	for _, key := range keys {
		if _, ok := values[key]; ok {
			return true
		}
	}
	return false
}
