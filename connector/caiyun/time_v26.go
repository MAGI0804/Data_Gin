package caiyun

import (
	"fmt"
	"strings"
	"time"
)

const caiyunISOTimeWithoutSeconds = "2006-01-02T15:04Z07:00"

func parseCaiyunISOTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	for _, layout := range []string{time.RFC3339Nano, caiyunISOTimeWithoutSeconds} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("caiyun time: invalid iso timestamp")
}
