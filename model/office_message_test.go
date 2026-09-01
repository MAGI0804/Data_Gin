package model

import "testing"

func TestOfficeMessageTableNames(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "messages", got: (OfficeMessage{}).TableName(), want: "office_messages"},
		{name: "targets", got: (OfficePushTarget{}).TableName(), want: "office_push_targets"},
		{name: "runs", got: (OfficePushRun{}).TableName(), want: "office_push_runs"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.got != test.want {
				t.Fatalf("TableName() = %q, want %q", test.got, test.want)
			}
		})
	}
}
