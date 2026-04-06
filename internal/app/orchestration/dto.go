package orchestration

import domain "go-sse-skeleton/internal/domain/orchestration"

type ExecuteGraphCommand struct {
	Input domain.GraphInput
}

type ExecuteGraphResult struct {
	Output domain.GraphResult
}
