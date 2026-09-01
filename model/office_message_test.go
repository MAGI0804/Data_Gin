package model

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestOfficeMessageTableNames(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "messages", got: (OfficeMessage{}).TableName(), want: "office_messages"},
		{name: "targets", got: (OfficePushTarget{}).TableName(), want: "office_push_targets"},
		{name: "runs", got: (OfficePushRun{}).TableName(), want: "office_push_runs"},
		{name: "procedure locks", got: (OfficeProcedureExportLock{}).TableName(), want: "office_procedure_export_locks"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.got != test.want {
				t.Fatalf("TableName() = %q, want %q", test.got, test.want)
			}
		})
	}
}

func TestOfficePushRunDoesNotMarshalInternalExecutionState(t *testing.T) {
	encoded, err := json.Marshal(OfficePushRun{ParametersJSON: JSONText(`{"bill_date":"20260901"}`), SnapshotJSON: JSONText(`{"receiveId":"private"}`), LeaseToken: "lease-private"})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if strings.Contains(string(encoded), "20260901") || strings.Contains(string(encoded), "private") {
		t.Fatalf("OfficePushRun leaked internal execution state: %s", encoded)
	}
}
