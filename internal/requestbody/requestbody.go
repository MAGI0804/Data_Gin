package requestbody

type Email_code_send struct {
	Email string `json:"email" binding:"required"`
}
