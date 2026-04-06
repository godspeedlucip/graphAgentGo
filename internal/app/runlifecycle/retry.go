package runlifecycle

import (
	"context"
	"time"

	domain "go-sse-skeleton/internal/domain/runlifecycle"
)

type RetryPolicy struct {
	MaxAttempts int
	Backoff     time.Duration
}

func defaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts: 1,
		Backoff:     100 * time.Millisecond,
	}
}

func (p RetryPolicy) ShouldRetry(status domain.Status, attempt int, hasDelta bool) bool {
	if p.MaxAttempts <= 1 {
		return false
	}
	if hasDelta {
		// Avoid duplicate output replay without resume cursor/outbox checkpoint.
		return false
	}
	if status == domain.StatusCanceled || status == domain.StatusTimeout {
		return false
	}
	return attempt < p.MaxAttempts
}

func (p RetryPolicy) Sleep(ctx context.Context) error {
	timer := time.NewTimer(p.Backoff)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
