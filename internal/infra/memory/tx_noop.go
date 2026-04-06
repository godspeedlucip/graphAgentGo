package memory

import "context"

type NoopTxManager struct{}

func NewNoopTxManager() *NoopTxManager { return &NoopTxManager{} }

func (m *NoopTxManager) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if fn == nil {
		return nil
	}
	return fn(ctx)
}
