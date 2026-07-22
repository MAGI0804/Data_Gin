package requestbody

import "time"

type MallWeatherHourlyQueryRequest struct {
	StartUTC      time.Time
	EndUTC        time.Time
	TimeZone      string
	Latest        bool
	AsOfUTC       *time.Time
	QualityStatus string
	Cursor        string
	PageSize      int
}

type MallWeatherRealtimeQueryRequest struct {
	StartUTC      time.Time
	EndUTC        time.Time
	TimeZone      string
	Latest        bool
	AsOfUTC       *time.Time
	QualityStatus string
	Cursor        string
	PageSize      int
}

type MallWeatherMinutelyQueryRequest struct {
	StartUTC      time.Time
	EndUTC        time.Time
	TimeZone      string
	Latest        bool
	AsOfUTC       *time.Time
	QualityStatus string
	Cursor        string
	PageSize      int
}

type MallWeatherDailyQueryRequest struct {
	StartUTC      time.Time
	EndUTC        time.Time
	TimeZone      string
	Latest        bool
	AsOfUTC       *time.Time
	QualityStatus string
	Cursor        string
	PageSize      int
}

type MallWeatherAlertQueryRequest struct {
	StartUTC      time.Time
	EndUTC        time.Time
	TimeZone      string
	Latest        bool
	AsOfUTC       *time.Time
	QualityStatus string
	Cursor        string
	PageSize      int
}

type MallWeatherLifeIndexQueryRequest struct {
	StartUTC      time.Time
	EndUTC        time.Time
	TimeZone      string
	Latest        bool
	AsOfUTC       *time.Time
	QualityStatus string
	Cursor        string
	PageSize      int
}

type MallWeatherFetchRunQueryRequest struct {
	StartUTC     time.Time
	EndUTC       time.Time
	TimeZone     string
	TaskKind     string
	EndpointKind string
	Status       string
	Cursor       string
	PageSize     int
}

type MallWeatherRefreshRequest struct {
	Kinds  []string `json:"kinds"`
	Force  bool     `json:"force"`
	Reason string   `json:"reason"`
}
