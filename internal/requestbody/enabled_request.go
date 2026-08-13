package requestbody

// EnabledUpdateRequest represents a partial status update. The pointer keeps
// false distinct from a missing field during request validation.
type EnabledUpdateRequest struct {
	Enabled *bool `json:"enabled" binding:"required"`
}
