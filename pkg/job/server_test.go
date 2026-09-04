package job

import (
	"errors"
	"testing"
	"time"

	"github.com/hibiken/asynq"
)

type fixedRetryDelayError struct{ delay time.Duration }

func (err fixedRetryDelayError) Error() string             { return "retry later" }
func (err fixedRetryDelayError) RetryDelay() time.Duration { return err.delay }

func TestRetryDelayUsesErrorHint(t *testing.T) {
	task := asynq.NewTask("report:run", nil)
	got := retryDelay(7, errors.Join(errors.New("wrapped"), fixedRetryDelayError{delay: 15 * time.Second}), task)
	if got != 15*time.Second {
		t.Fatalf("retryDelay() = %s, want 15s", got)
	}
}
