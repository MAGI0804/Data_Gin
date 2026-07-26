package job

import "testing"

func TestExcelMatchCleanupTaskUsesStrictEmptyPayload(t *testing.T) {
	task, err := NewExcelMatchCleanupTask()
	if err != nil {
		t.Fatalf("NewExcelMatchCleanupTask() error=%v", err)
	}
	if task.Type() != TypeExcelMatchCleanup || string(task.Payload()) != `{}` {
		t.Fatalf("cleanup task type=%q payload=%s", task.Type(), task.Payload())
	}
	for _, invalid := range [][]byte{
		nil,
		[]byte(`null`),
		[]byte(`{"job_id":1}`),
		[]byte(`{}{}`),
	} {
		if err := DecodeExcelMatchCleanupTaskPayload(invalid); err == nil {
			t.Fatalf("DecodeExcelMatchCleanupTaskPayload(%s) accepted invalid payload", invalid)
		}
	}
}
