package job

import "testing"

func TestDecodeOfficePushTaskPayload(t *testing.T) {
	payload, err := DecodeOfficePushTaskPayload([]byte(`{"run_id":17}`))
	if err != nil || payload.RunID != 17 {
		t.Fatalf("DecodeOfficePushTaskPayload() = %#v, %v", payload, err)
	}
	for _, invalid := range [][]byte{[]byte(`{}`), []byte(`{"run_id":0}`), []byte(`{"run_id":1,"secret":"x"}`), []byte(`{"run_id":1}{}`)} {
		if _, err := DecodeOfficePushTaskPayload(invalid); err == nil {
			t.Fatalf("DecodeOfficePushTaskPayload(%s) accepted invalid payload", invalid)
		}
	}
}
