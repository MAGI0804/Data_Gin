package requestbody

type OpenWeatherMallQueryRequest struct {
	Cursor   string `json:"cursor"`
	PageSize int    `json:"pageSize"`
}
