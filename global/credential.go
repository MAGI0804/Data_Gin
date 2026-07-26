package global

import "gin-biz-web-api/pkg/credential"

// Credentials contains weather and integration credentials loaded once during bootstrap.
// Its fields are private and cannot be serialized by configuration backups.
var Credentials credential.Config
