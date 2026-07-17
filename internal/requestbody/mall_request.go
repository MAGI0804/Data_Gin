package requestbody

type MallBusinessHour struct {
	Open  string `json:"open"`
	Close string `json:"close"`
}

type MallWeatherSettingsRequest struct {
	Enabled         *bool   `json:"enabled,omitempty"`
	DetailProfile   *string `json:"detailProfile,omitempty"`
	CoverageRadiusM *int    `json:"coverageRadiusM,omitempty"`
}

type MallCreateRequest struct {
	MallCode          string                        `json:"mallCode"`
	NameCN            string                        `json:"nameCn"`
	NameEN            string                        `json:"nameEn,omitempty"`
	Province          string                        `json:"province"`
	City              string                        `json:"city"`
	District          string                        `json:"district,omitempty"`
	Address           string                        `json:"address"`
	BusinessHours     map[string][]MallBusinessHour `json:"businessHours,omitempty"`
	GrossFloorAreaSQM *float64                      `json:"grossFloorAreaSqm,omitempty"`
	ParkingSpaces     *int                          `json:"parkingSpaces,omitempty"`
	Tags              []string                      `json:"tags,omitempty"`
	Weather           MallWeatherSettingsRequest    `json:"weather,omitempty"`
}

type MallPatchRequest struct {
	ExpectedMallVersion uint64                         `json:"expectedMallVersion"`
	NameCN              *string                        `json:"nameCn,omitempty"`
	NameEN              *string                        `json:"nameEn,omitempty"`
	Province            *string                        `json:"province,omitempty"`
	City                *string                        `json:"city,omitempty"`
	District            *string                        `json:"district,omitempty"`
	Address             *string                        `json:"address,omitempty"`
	BusinessHours       *map[string][]MallBusinessHour `json:"businessHours,omitempty"`
	GrossFloorAreaSQM   *float64                       `json:"grossFloorAreaSqm,omitempty"`
	ParkingSpaces       *int                           `json:"parkingSpaces,omitempty"`
	Tags                *[]string                      `json:"tags,omitempty"`
	Weather             *MallWeatherSettingsRequest    `json:"weather,omitempty"`
}

type MallListRequest struct {
	AfterID        uint
	Limit          int
	City           string
	Status         string
	GeocodeStatus  string
	WeatherEnabled *bool
}
