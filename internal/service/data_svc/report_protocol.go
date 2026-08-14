package data_svc

import (
	"strings"

	"gin-biz-web-api/model"
)

func isJSONTableSnapshot(version model.ReportVersion) bool {
	return version.ExecutionMode == model.ReportExecutionModeTableSnapshot && strings.TrimSpace(version.JSONInputArgName) != ""
}

func isJSONInputReport(version model.ReportVersion) bool {
	return version.ExecutionMode == model.ReportExecutionModeRefCursor || isJSONTableSnapshot(version)
}
