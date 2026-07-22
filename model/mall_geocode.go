package model

import "time"

type MallGeocodeRun struct {
	BaseModel
	MallID              uint       `gorm:"column:mall_id;not null;index" json:"mall_id"`
	RequestAddress      string     `gorm:"column:request_address;size:1000;not null" json:"request_address"`
	RequestCity         string     `gorm:"column:request_city;size:128" json:"request_city"`
	AddressHash         string     `gorm:"column:address_hash;type:char(64);not null;index" json:"address_hash"`
	ProviderStatus      string     `gorm:"column:provider_status;size:32" json:"provider_status"`
	Infocode            string     `gorm:"column:infocode;size:32" json:"infocode"`
	Info                string     `gorm:"column:info;size:255" json:"info"`
	CandidateCount      int        `gorm:"column:candidate_count;not null;default:0" json:"candidate_count"`
	SelectedCandidateID *uint      `gorm:"column:selected_candidate_id" json:"selected_candidate_id"`
	RawSnapshotID       *uint      `gorm:"column:raw_snapshot_id;index" json:"raw_snapshot_id"`
	Status              string     `gorm:"column:status;size:32;not null;index" json:"status"`
	ErrorClass          string     `gorm:"column:error_class;size:64" json:"error_class"`
	ErrorCode           string     `gorm:"column:error_code;size:64" json:"error_code"`
	ErrorMessageSafe    string     `gorm:"column:error_message_safe;type:text" json:"error_message_safe"`
	StartedAt           time.Time  `gorm:"column:started_at;type:datetime(3);not null;index" json:"started_at"`
	FinishedAt          *time.Time `gorm:"column:finished_at;type:datetime(3)" json:"finished_at"`
	DurationMS          int64      `gorm:"column:duration_ms;not null;default:0" json:"duration_ms"`
	CreatedBy           uint       `gorm:"column:created_by;default:0" json:"created_by"`
	WeatherTimestamps
}

func (MallGeocodeRun) TableName() string { return "mall_geocode_runs" }

type MallGeocodeCandidate struct {
	BaseModel
	RunID            uint     `gorm:"column:run_id;not null;uniqueIndex:uk_geocode_candidate,priority:1;index" json:"run_id"`
	MallID           uint     `gorm:"column:mall_id;not null;index" json:"mall_id"`
	CandidateNo      int      `gorm:"column:candidate_no;not null;uniqueIndex:uk_geocode_candidate,priority:2" json:"candidate_no"`
	Country          string   `gorm:"column:country;size:128" json:"country"`
	Province         string   `gorm:"column:province;size:128" json:"province"`
	City             string   `gorm:"column:city;size:128" json:"city"`
	Citycode         string   `gorm:"column:citycode;size:32" json:"citycode"`
	District         string   `gorm:"column:district;size:128" json:"district"`
	Adcode           string   `gorm:"column:adcode;size:32;index" json:"adcode"`
	Township         string   `gorm:"column:township;size:128" json:"township"`
	Street           string   `gorm:"column:street;size:255" json:"street"`
	StreetNumber     string   `gorm:"column:street_number;size:64" json:"street_number"`
	FormattedAddress string   `gorm:"column:formatted_address;size:1000" json:"formatted_address"`
	Longitude        float64  `gorm:"column:longitude;type:decimal(10,7);not null" json:"longitude"`
	Latitude         float64  `gorm:"column:latitude;type:decimal(10,7);not null" json:"latitude"`
	CoordinateSystem string   `gorm:"column:coordinate_system;size:16;not null;default:'GCJ02'" json:"coordinate_system"`
	Level            string   `gorm:"column:level;size:64" json:"level"`
	ConfidenceScore  float64  `gorm:"column:confidence_score;type:decimal(5,2);not null;default:0" json:"confidence_score"`
	ScoreReasonsJSON JSONText `gorm:"column:score_reasons_json;type:json" json:"score_reasons_json"`
	IsSelected       bool     `gorm:"column:is_selected;not null;default:false;index" json:"is_selected"`
	WeatherTimestamps
}

func (MallGeocodeCandidate) TableName() string { return "mall_geocode_candidates" }

// MallCoordinateAudit is an append-only record of every operator-confirmed
// coordinate change. Nullable before values preserve the first confirmation,
// while run and candidate references retain the provenance of provider data.
type MallCoordinateAudit struct {
	BaseModel
	MallID                 uint      `gorm:"column:mall_id;not null;index:idx_mall_coordinate_audit,priority:1" json:"mall_id"`
	RunID                  *uint     `gorm:"column:run_id;index" json:"run_id"`
	CandidateID            *uint     `gorm:"column:candidate_id;index" json:"candidate_id"`
	Source                 string    `gorm:"column:source;size:32;not null" json:"source"`
	BeforeLongitude        *float64  `gorm:"column:before_longitude;type:decimal(10,7)" json:"before_longitude"`
	BeforeLatitude         *float64  `gorm:"column:before_latitude;type:decimal(10,7)" json:"before_latitude"`
	BeforeCoordinateSystem string    `gorm:"column:before_coordinate_system;size:16" json:"before_coordinate_system"`
	AfterLongitude         *float64  `gorm:"column:after_longitude;type:decimal(10,7)" json:"after_longitude"`
	AfterLatitude          *float64  `gorm:"column:after_latitude;type:decimal(10,7)" json:"after_latitude"`
	AfterCoordinateSystem  string    `gorm:"column:after_coordinate_system;size:16" json:"after_coordinate_system"`
	Reason                 string    `gorm:"column:reason;size:500" json:"reason"`
	ConfirmedBy            uint      `gorm:"column:confirmed_by;not null;index" json:"confirmed_by"`
	MallVersionBefore      uint64    `gorm:"column:mall_version_before;not null" json:"mall_version_before"`
	MallVersionAfter       uint64    `gorm:"column:mall_version_after;not null" json:"mall_version_after"`
	ConfirmedAt            time.Time `gorm:"column:confirmed_at;type:datetime(3);not null;index:idx_mall_coordinate_audit,priority:2" json:"confirmed_at"`
	CreatedAt              time.Time `gorm:"column:created_at;type:datetime(3);not null;autoCreateTime" json:"created_at"`
}

func (MallCoordinateAudit) TableName() string { return "mall_coordinate_audits" }
