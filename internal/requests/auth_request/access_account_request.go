package auth_request

type AccessAccountQueryRequest struct {
	Keyword  string `json:"keyword"`
	Status   string `json:"status"`
	BeforeID uint   `json:"beforeId"`
	PageSize int    `json:"pageSize"`
}

type AccessAccountCreateRequest struct {
	Account       string `json:"account"`
	Phone         string `json:"phone"`
	Nickname      string `json:"nickname"`
	Password      string `json:"password"`
	RoleIDs       []uint `json:"roleIds"`
	MallScopeMode string `json:"mallScopeMode"`
	MallIDs       []uint `json:"mallIds"`
	Reason        string `json:"reason"`
}

type AccessAccountUpdateRequest struct {
	Phone    string `json:"phone"`
	Nickname string `json:"nickname"`
	Reason   string `json:"reason"`
}

type AccessAccountStatusRequest struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}

type AccessAccountPasswordResetRequest struct {
	Password string `json:"password"`
	Reason   string `json:"reason"`
}

type AccessAccountRolesRequest struct {
	RoleIDs []uint `json:"roleIds"`
	Reason  string `json:"reason"`
}

type AccessAccountMallScopeRequest struct {
	MallScopeMode string `json:"mallScopeMode"`
	MallIDs       []uint `json:"mallIds"`
	Reason        string `json:"reason"`
}
