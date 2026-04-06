package runlifecycle

import (
	"context"
	"time"

	domain "go-sse-skeleton/internal/domain/runlifecycle"
)

type NoopLifecyclePublisher struct{}

func NewNoopLifecyclePublisher() *NoopLifecyclePublisher {
	return &NoopLifecyclePublisher{}
}

func (p *NoopLifecyclePublisher) PublishLifecycle(_ context.Context, _ domain.LifecycleEvent) error {
	return nil
}

type NoopSSENotifier struct{}

func NewNoopSSENotifier() *NoopSSENotifier {
	return &NoopSSENotifier{}
}

func (n *NoopSSENotifier) NotifyStarted(_ context.Context, _ string, _ string) error { return nil }
func (n *NoopSSENotifier) NotifyDelta(_ context.Context, _ string, _ string, _ string) error {
	return nil
}
func (n *NoopSSENotifier) NotifyDone(_ context.Context, _ string, _ string) error    { return nil }
func (n *NoopSSENotifier) NotifyFailed(_ context.Context, _ string, _ string, _ string) error {
	return nil
}

type SystemClock struct{}

func NewSystemClock() *SystemClock {
	return &SystemClock{}
}

func (SystemClock) Now() time.Time {
	return time.Now()
}
