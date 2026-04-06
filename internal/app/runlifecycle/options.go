package runlifecycle

type Option func(*service)

func WithRetryPolicy(policy RetryPolicy) Option {
	return func(s *service) {
		if policy.MaxAttempts <= 0 {
			policy.MaxAttempts = 1
		}
		if policy.Backoff < 0 {
			policy.Backoff = 0
		}
		s.retryPolicy = policy
	}
}

func WithRuntimeRegistry(registry RuntimeRegistry) Option {
	return func(s *service) {
		if registry != nil {
			s.registry = registry
		}
	}
}
