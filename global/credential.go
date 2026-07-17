package global

import "gin-biz-web-api/pkg/credential"

// Credentials is loaded once during bootstrap from the process environment.
// Its fields are private and cannot be serialized by configuration backups.
var Credentials credential.Config
