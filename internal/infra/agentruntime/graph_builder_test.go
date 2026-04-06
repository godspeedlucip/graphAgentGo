package agentruntime

import (
	"context"
	"strings"
	"testing"

	chatdomain "go-sse-skeleton/internal/domain/chat"
	port "go-sse-skeleton/internal/port/agentruntime"
)

type streamModel struct {
	streamed []string
}

func (m *streamModel) Generate(context.Context, string) (string, error) {
	return "fallback", nil
}

func (m *streamModel) GenerateStream(_ context.Context, _ string, onToken func(token string) error) error {
	for _, tk := range []string{"A", "B", "C"} {
		m.streamed = append(m.streamed, tk)
		if err := onToken(tk); err != nil {
			return err
		}
	}
	return nil
}

func TestCompiledGraphDAGAndStreamingHooks(t *testing.T) {
	t.Parallel()

	builder := NewPlaceholderGraphBuilder()
	model := &streamModel{}
	graph, err := builder.Build(context.Background(), port.GraphBuildInput{
		AgentID:      "agent-1",
		SessionID:    "s1",
		SystemPrompt: "you are helpful",
		Tools:        []string{"tool-1"},
		ModelClient:  model,
	})
	if err != nil {
		t.Fatalf("build graph: %v", err)
	}

	var (
		appended []string
		collected strings.Builder
	)
	ctx := port.WithExecutionHooks(context.Background(), port.ExecutionHooks{
		AssistantMessageID: "assistant-1",
		RecentMessages: []*chatdomain.Message{
			{Role: chatdomain.RoleUser, Content: "what is go"},
		},
		AppendDelta: func(_ context.Context, delta string) error {
			appended = append(appended, delta)
			return nil
		},
		CollectDelta: func(delta string) {
			_, _ = collected.WriteString(delta)
		},
	})

	if err = graph.Execute(ctx); err != nil {
		t.Fatalf("execute graph: %v", err)
	}
	if strings.Join(appended, "") != "ABC" {
		t.Fatalf("unexpected streamed tokens: %+v", appended)
	}
	if collected.String() != "ABC" {
		t.Fatalf("unexpected collected output: %q", collected.String())
	}
}
