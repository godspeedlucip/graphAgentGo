package sse

import "context"

type MessageNotifier interface {
	NotifyDone(ctx context.Context, sessionID string) error
}