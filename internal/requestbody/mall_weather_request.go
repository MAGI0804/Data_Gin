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
	IncludeTotals bool
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
	IncludeTotals bool
}

type OpenWeatherHistoryDayQueryRequest struct {
	Date          string
	TimeZone      string
	QualityStatus string
	Cursor        string
	PageSize      int
}

type OpenWeatherHistoryDaySummaryRequest struct {
	Date          string
	TimeZone      string
	QualityStatus string
}

type OpenWeatherHistoryRangeQueryRequest struct {
	StartTime     string
	EndTime       string
	TimeZone      string
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
	IncludeTotals bool
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
	IncludeTotals bool
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
	IncludeTotals bool
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
	IncludeTotals bool
}

type MallWeatherFetchRunQueryRequest struct {
	StartUTC      time.Time
	EndUTC        time.Time
	TimeZone      string
	CorrelationID string
	TaskKind      string
	EndpointKind  string
	Status        string
	Cursor        string
	PageSize      int
}

type MallWeatherRefreshRequest struct {
	Kinds  []string `json:"kinds"`
	Force  bool     `json:"force"`
	Reason string   `json:"reason"`
}

type MallWeatherExportColumn struct {
	Field  string  `json:"field"`
	Title  string  `json:"title"`
	Width  float64 `json:"width,omitempty"`
	Format string  `json:"format,omitempty"`
}

type MallWeatherExportConditionalFormat struct {
	Field           string   `json:"field"`
	Operator        string   `json:"operator"`
	Value           *float64 `json:"value,omitempty"`
	SecondValue     *float64 `json:"secondValue,omitempty"`
	BackgroundColor string   `json:"backgroundColor,omitempty"`
	FontColor       string   `json:"fontColor,omitempty"`
}

type MallWeatherExportFilters struct {
	MallIDs         []uint   `json:"mallIds,omitempty"`
	Cities          []string `json:"cities,omitempty"`
	MallStatuses    []string `json:"mallStatuses,omitempty"`
	QualityStatuses []string `json:"qualityStatuses,omitempty"`
	Start           string   `json:"start,omitempty"`
	End             string   `json:"end,omitempty"`
}

type MallWeatherExportDataset struct {
	Kind               string                               `json:"kind"`
	SheetName          string                               `json:"sheetName"`
	Columns            []MallWeatherExportColumn            `json:"columns,omitempty"`
	Latest             *bool                                `json:"latest,omitempty"`
	AsOf               string                               `json:"asOf,omitempty"`
	SplitBy            string                               `json:"splitBy,omitempty"`
	FreezeHeader       bool                                 `json:"freezeHeader"`
	AutoFilter         bool                                 `json:"autoFilter"`
	MaxRows            int                                  `json:"maxRows,omitempty"`
	ConditionalFormats []MallWeatherExportConditionalFormat `json:"conditionalFormats,omitempty"`
}

type MallWeatherExportProfileSaveRequest struct {
	Code             string                     `json:"code"`
	Name             string                     `json:"name"`
	ExpectedVersion  *uint64                    `json:"expectedVersion,omitempty"`
	Enabled          *bool                      `json:"enabled,omitempty"`
	TimeZone         string                     `json:"timeZone"`
	UnitSystem       string                     `json:"unitSystem,omitempty"`
	DateFormat       string                     `json:"dateFormat,omitempty"`
	DateTimeFormat   string                     `json:"dateTimeFormat,omitempty"`
	FileNameTemplate string                     `json:"fileNameTemplate"`
	Filters          MallWeatherExportFilters   `json:"filters,omitempty"`
	Datasets         []MallWeatherExportDataset `json:"datasets"`
}

type MallWeatherExportCreateRequest struct {
	ProfileID              uint                      `json:"profileId,omitempty"`
	ExpectedProfileVersion *uint64                   `json:"expectedProfileVersion,omitempty"`
	Filters                *MallWeatherExportFilters `json:"filters,omitempty"`
}

type MallWeatherFeishuPushRequest struct {
	DestinationID          uint                      `json:"destinationId"`
	ProfileID              uint                      `json:"profileId"`
	ExpectedProfileVersion *uint64                   `json:"expectedProfileVersion,omitempty"`
	Filters                *MallWeatherExportFilters `json:"filters,omitempty"`
}
