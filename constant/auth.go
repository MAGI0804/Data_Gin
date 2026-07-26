package constant

import "strings"

const (
	CurrentUserInfo  = "current_user_info"
	CurrentUserID    = "current_user_id"
	ConsoleAdmin     = "admin"
	ConsoleAdminMail = "admin@warehouse.local"
	ConsoleAdminName = "管理员"
)

// IsConsoleAdminAccount reserves the console administrator identity from all
// public registration paths. MySQL account uniqueness is commonly
// case-insensitive, so the boundary must be case-insensitive too.
func IsConsoleAdminAccount(account string) bool {
	return strings.EqualFold(strings.TrimSpace(account), ConsoleAdmin)
}
