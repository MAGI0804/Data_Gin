package reportidentity

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"gin-biz-web-api/model"
)

const (
	BindingIdentitySourceOracle = "ORACLE_DATABASE_V1"
	BindingIdentitySourceLegacy = "LEGACY_DATASOURCE_V1"
)

type OracleDatabaseIdentity struct {
	DBID          string
	DBUniqueName  string
	DBName        string
	ContainerID   string
	ContainerUID  string
	ContainerName string
}

// DatasourceFingerprint returns a stable identity for the physical Oracle
// account. Passwords and operational pool settings are deliberately excluded.
func DatasourceFingerprint(datasource model.ReportDatasource) string {
	parts := []string{
		strings.ToUpper(strings.TrimSpace(datasource.Driver)),
		strings.ToLower(strings.TrimSpace(datasource.Host)),
		strconv.Itoa(datasource.Port),
		strings.ToUpper(strings.TrimSpace(datasource.ServiceName)),
		strings.ToUpper(strings.TrimSpace(datasource.SID)),
		strings.ToUpper(strings.TrimSpace(datasource.Username)),
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x1f")))
	return hex.EncodeToString(sum[:])
}

// OracleDatabaseFingerprint identifies the database and current CDB/PDB
// container reported by Oracle itself. Connection aliases and login accounts
// are deliberately excluded because neither changes the physical result table.
func OracleDatabaseFingerprint(identity OracleDatabaseIdentity) (string, error) {
	dbid := normalizeDecimalIdentity(identity.DBID)
	uniqueName := strings.ToUpper(strings.TrimSpace(identity.DBUniqueName))
	containerUID := normalizeDecimalIdentity(identity.ContainerUID)
	containerName := strings.ToUpper(strings.TrimSpace(identity.ContainerName))
	if dbid == "" && uniqueName == "" {
		return "", fmt.Errorf("oracle database identity requires DBID or DB_UNIQUE_NAME")
	}
	databaseKey := "DB_UNIQUE_NAME:" + uniqueName
	if dbid != "" {
		databaseKey = "DBID:" + dbid
	}
	containerKey := "NON_CDB"
	if containerUID != "" && containerUID != "0" {
		containerKey = "CON_UID:" + containerUID
	} else if containerName != "" && containerName != "CDB$ROOT" {
		containerKey = "CON_NAME:" + containerName
	}
	parts := []string{BindingIdentitySourceOracle, databaseKey, containerKey}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x1f")))
	return hex.EncodeToString(sum[:]), nil
}

func normalizeDecimalIdentity(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return ""
		}
	}
	value = strings.TrimLeft(value, "0")
	if value == "" {
		return "0"
	}
	return value
}
