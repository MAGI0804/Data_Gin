package model

import "time"

const (
	DataAuthorizationActionAccountCreate = "ACCOUNT_CREATE"
	DataAuthorizationActionGrant         = "GRANT"
	DataAuthorizationActionRenew         = "RENEW"
	DataAuthorizationActionRevoke        = "REVOKE"
	DataAuthorizationActionTokenReissue  = "TOKEN_REISSUE"

	OpenAPICredentialStatusActive  = "ACTIVE"
	OpenAPICredentialStatusRevoked = "REVOKED"
)

// OpenAPICredential stores only a one-way token hash. Plaintext credentials
// are returned once by the issuing service and never persisted.
type OpenAPICredential struct {
	BaseModel
	UserID      uint       `gorm:"column:user_id;not null;index:idx_open_api_credential_user_status,priority:1" json:"userId"`
	TokenHash   string     `gorm:"column:token_hash;type:char(64);not null;uniqueIndex:uk_open_api_credential_hash" json:"-"`
	TokenPrefix string     `gorm:"column:token_prefix;size:20;not null" json:"tokenPrefix"`
	Status      string     `gorm:"column:status;size:16;not null;index:idx_open_api_credential_user_status,priority:2" json:"status"`
	IssuedBy    uint       `gorm:"column:issued_by;not null;index" json:"issuedBy"`
	IssuedAt    time.Time  `gorm:"column:issued_at;type:datetime(3);not null;index" json:"issuedAt"`
	RevokedAt   *time.Time `gorm:"column:revoked_at;type:datetime(3)" json:"revokedAt,omitempty"`
	WeatherTimestamps
}

func (OpenAPICredential) TableName() string { return "open_api_credentials" }

// DataAuthorizationAudit is append-only evidence for every account and grant
// mutation performed by a trusted console administrator.
type DataAuthorizationAudit struct {
	BaseModel
	TargetUserID       uint       `gorm:"column:target_user_id;not null;index:idx_data_auth_target_created,priority:1" json:"targetUserId"`
	Permission         string     `gorm:"column:permission;size:64;not null;index:idx_data_auth_permission_created,priority:1" json:"permission"`
	Action             string     `gorm:"column:action;size:32;not null;index" json:"action"`
	OldExpiresAt       *time.Time `gorm:"column:old_expires_at;type:datetime(3)" json:"oldExpiresAt,omitempty"`
	NewExpiresAt       *time.Time `gorm:"column:new_expires_at;type:datetime(3)" json:"newExpiresAt,omitempty"`
	ActorUserID        uint       `gorm:"column:actor_user_id;not null;index:idx_data_auth_actor_created,priority:1" json:"actorUserId"`
	Reason             string     `gorm:"column:reason;size:500;not null" json:"reason"`
	IdempotencyKeyHash string     `gorm:"column:idempotency_key_hash;type:char(64);not null" json:"-"`
	WeatherTimestamps
}

func (DataAuthorizationAudit) TableName() string { return "data_authorization_audits" }
