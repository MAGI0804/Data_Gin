package requestbody

// OpenBojunOrderQueryRequest is the public filter contract for querying
// sanitized Bojun order details.
type OpenBojunOrderQueryRequest struct {
	StartDate  string   `json:"startDate"`
	EndDate    string   `json:"endDate"`
	StoreCodes []string `json:"storeCodes"`
	OrderTypes []string `json:"orderTypes"`
	Cursor     string   `json:"cursor"`
	PageSize   int      `json:"pageSize"`
}
