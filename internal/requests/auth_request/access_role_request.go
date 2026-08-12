package auth_request

type AccessRoleCreateRequest struct {
	Code        string   `json:"code"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
	Reason      string   `json:"reason"`
}

type AccessRoleUpdateRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Reason      string `json:"reason"`
}

type AccessRoleStatusRequest struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}

type AccessRolePermissionsRequest struct {
	Permissions []string `json:"permissions"`
	Reason      string   `json:"reason"`
}

type AccessRoleDeleteRequest struct {
	Reason string `json:"reason"`
}

type AccessAuditQueryRequest struct {
	BeforeID   uint   `json:"beforeId" form:"beforeId"`
	PageSize   int    `json:"pageSize" form:"pageSize"`
	ActorID    uint   `json:"actorId" form:"actorId"`
	Action     string `json:"action" form:"action"`
	TargetType string `json:"targetType" form:"targetType"`
	TargetID   uint   `json:"targetId" form:"targetId"`
	StartTime  string `json:"startTime" form:"startTime"`
	EndTime    string `json:"endTime" form:"endTime"`
}
