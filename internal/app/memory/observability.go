package memory

type Observer interface {
	RecordGetSource(source string)
	RecordFallback(reason string)
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

func (n *NoopObserver) RecordGetSource(string) {}

func (n *NoopObserver) RecordFallback(string) {}
