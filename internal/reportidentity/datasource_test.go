package reportidentity

import (
	"testing"

	"gin-biz-web-api/model"
)

func TestDatasourceFingerprintNormalizesPhysicalIdentity(t *testing.T) {
	base := model.ReportDatasource{Driver: "ORACLE", Host: "Db.Internal", Port: 1521, ServiceName: "report_pdb", Username: "report_user"}
	normalized := base
	normalized.Driver = " oracle "
	normalized.Host = " db.internal "
	normalized.ServiceName = " REPORT_PDB "
	normalized.Username = " REPORT_USER "
	normalized.PasswordCiphertext = "different-password"
	normalized.MaxOpenConnections = 99
	if got, want := DatasourceFingerprint(normalized), DatasourceFingerprint(base); got != want {
		t.Fatalf("normalized fingerprint = %q, want %q", got, want)
	}

	changed := base
	changed.Username = "other_user"
	if DatasourceFingerprint(changed) == DatasourceFingerprint(base) {
		t.Fatal("different Oracle accounts produced the same fingerprint")
	}
}

func TestOracleDatabaseFingerprintIgnoresConnectionAliasesAndUsers(t *testing.T) {
	identity := OracleDatabaseIdentity{DBID: "0012345", DBUniqueName: "REPORT_PRIMARY", ContainerUID: "0099", ContainerName: "REPORT_PDB"}
	fingerprint, err := OracleDatabaseFingerprint(identity)
	if err != nil || len(fingerprint) != 64 {
		t.Fatalf("OracleDatabaseFingerprint() = %q, %v", fingerprint, err)
	}
	alias := identity
	alias.DBUniqueName = "report_standby"
	alias.ContainerName = "report_pdb_alias"
	if got, err := OracleDatabaseFingerprint(alias); err != nil || got != fingerprint {
		t.Fatalf("DBID/CON_UID alias fingerprint = %q, %v, want %q", got, err, fingerprint)
	}
}

func TestOracleDatabaseFingerprintSeparatesPDBContainers(t *testing.T) {
	first := OracleDatabaseIdentity{DBID: "12345", ContainerUID: "101", ContainerName: "REPORT_A"}
	second := OracleDatabaseIdentity{DBID: "12345", ContainerUID: "102", ContainerName: "REPORT_B"}
	firstFingerprint, firstErr := OracleDatabaseFingerprint(first)
	secondFingerprint, secondErr := OracleDatabaseFingerprint(second)
	if firstErr != nil || secondErr != nil || firstFingerprint == secondFingerprint {
		t.Fatalf("container fingerprints = %q/%q, errors=%v/%v", firstFingerprint, secondFingerprint, firstErr, secondErr)
	}
	if _, err := OracleDatabaseFingerprint(OracleDatabaseIdentity{}); err == nil {
		t.Fatal("missing Oracle database identity was accepted")
	}
}

func TestOracleDatabaseFingerprintUsesUniqueNameWithoutCatalogDBID(t *testing.T) {
	first, err := OracleDatabaseFingerprint(OracleDatabaseIdentity{DBUniqueName: " report_prod ", ContainerName: " REPORT_PDB "})
	if err != nil {
		t.Fatalf("OracleDatabaseFingerprint() error = %v", err)
	}
	second, err := OracleDatabaseFingerprint(OracleDatabaseIdentity{DBUniqueName: "REPORT_PROD", ContainerName: "REPORT_PDB"})
	if err != nil || first != second {
		t.Fatalf("normalized fallback fingerprints = %q/%q, error=%v", first, second, err)
	}
}
