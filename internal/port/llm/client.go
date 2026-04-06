package llm

import "context"

type Client interface {
	Generate(ctx context.Context, prompt string) (string, error)
}