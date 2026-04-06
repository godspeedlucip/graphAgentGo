package event

import "time"

type NoopObserver struct{}

func NewNoopObserver() *NoopObserver { return &NoopObserver{} }

func (n *NoopObserver) RecordPublished(string) {}

func (n *NoopObserver) RecordConsumed(string, time.Duration) {}

func (n *NoopObserver) RecordFailed(string, string) {}

func (n *NoopObserver) RecordQueueLen(string, int) {}
