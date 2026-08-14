package reportoracle

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

const databaseIdentitySQL = `
SELECT TO_CHAR(database_identity.dbid), database_identity.db_unique_name,
       SYS_CONTEXT('USERENV', 'DB_NAME'), SYS_CONTEXT('USERENV', 'CON_ID'),
       SYS_CONTEXT('USERENV', 'CON_UID'), SYS_CONTEXT('USERENV', 'CON_NAME')
FROM v$database database_identity`

const databaseIdentityFallbackSQL = `
SELECT NULL, SYS_CONTEXT('USERENV', 'DB_UNIQUE_NAME'),
       SYS_CONTEXT('USERENV', 'DB_NAME'), SYS_CONTEXT('USERENV', 'CON_ID'),
       SYS_CONTEXT('USERENV', 'CON_UID'), SYS_CONTEXT('USERENV', 'CON_NAME')
FROM dual`

type DatabaseIdentity struct {
	DBID          string
	DBUniqueName  string
	DBName        string
	ContainerID   string
	ContainerUID  string
	ContainerName string
}

// InspectDatabaseIdentity reads an alias-independent database identity. The
// USERENV fallback supports least-privilege report accounts without V$DATABASE
// access, while DBID remains preferred when the catalog grant is available.
func (adapter *Adapter) InspectDatabaseIdentity(ctx context.Context) (DatabaseIdentity, error) {
	if adapter == nil || adapter.db == nil {
		return DatabaseIdentity{}, fmt.Errorf("inspect oracle database identity: adapter is closed")
	}
	identity, primaryErr := queryDatabaseIdentity(ctx, adapter.db, databaseIdentitySQL)
	if primaryErr == nil {
		return validateDatabaseIdentity(identity)
	}
	identity, fallbackErr := queryDatabaseIdentity(ctx, adapter.db, databaseIdentityFallbackSQL)
	if fallbackErr != nil {
		return DatabaseIdentity{}, fmt.Errorf("inspect oracle database identity: %w", errors.Join(primaryErr, fallbackErr))
	}
	return validateDatabaseIdentity(identity)
}

type databaseIdentityQueryer interface {
	QueryRowContext(context.Context, string, ...interface{}) *sql.Row
}

func queryDatabaseIdentity(ctx context.Context, queryer databaseIdentityQueryer, statement string) (DatabaseIdentity, error) {
	var dbid, uniqueName, dbName, containerID, containerUID, containerName sql.NullString
	err := queryer.QueryRowContext(ctx, statement).Scan(&dbid, &uniqueName, &dbName, &containerID, &containerUID, &containerName)
	if err != nil {
		return DatabaseIdentity{}, err
	}
	return DatabaseIdentity{
		DBID: strings.TrimSpace(dbid.String), DBUniqueName: strings.TrimSpace(uniqueName.String),
		DBName: strings.TrimSpace(dbName.String), ContainerID: strings.TrimSpace(containerID.String),
		ContainerUID: strings.TrimSpace(containerUID.String), ContainerName: strings.TrimSpace(containerName.String),
	}, nil
}

func validateDatabaseIdentity(identity DatabaseIdentity) (DatabaseIdentity, error) {
	if strings.TrimSpace(identity.DBID) == "" && strings.TrimSpace(identity.DBUniqueName) == "" {
		return DatabaseIdentity{}, fmt.Errorf("%w: Oracle did not expose DBID or DB_UNIQUE_NAME", ErrMetadataMismatch)
	}
	return identity, nil
}
