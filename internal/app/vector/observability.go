package vector

type Observer interface {
	RecordFallback(scope string, reason string)
	RecordDimensionMismatch(queryDim int, chunkDim int)
}

type Option func(*service)

func WithObserver(observer Observer) Option {
	return func(s *service) {
		if observer != nil {
			s.observer = observer
		}
	}
}

type NoopObserver struct{}

func NewNoopObserver() *NoopObserver { return &NoopObserver{} }

func (n *NoopObserver) RecordFallback(string, string) {}

func (n *NoopObserver) RecordDimensionMismatch(int, int) {}
