package global

// ReportCenterEnabledAtStartup is captured after configuration validation.
// HTTP admission, workers, dispatchers, and scheduled cleanup must all use the
// same value because changing the report runtime requires a process restart.
var ReportCenterEnabledAtStartup bool
