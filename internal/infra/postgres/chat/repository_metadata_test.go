package chat

import (
	"testing"

	domain "go-sse-skeleton/internal/domain/chat"
)

func TestMetadataRoundTrip(t *testing.T) {
	t.Parallel()

	in := &domain.Metadata{
		ToolResponse: map[string]any{"status": "ok", "count": float64(2)},
		ToolCalls:    []any{"call-a", map[string]any{"name": "tool-x"}},
	}

	raw, err := toMetadataValue(in)
	if err != nil {
		t.Fatalf("toMetadataValue: %v", err)
	}

	round, err := parseMetadata(raw.(string))
	if err != nil {
		t.Fatalf("parseMetadata: %v", err)
	}
	if round == nil {
		t.Fatal("round-trip metadata is nil")
	}
	if len(round.ToolCalls) != 2 {
		t.Fatalf("unexpected toolCalls len: %d", len(round.ToolCalls))
	}
	resp, ok := round.ToolResponse.(map[string]any)
	if !ok {
		t.Fatalf("toolResponse type mismatch: %T", round.ToolResponse)
	}
	if resp["status"] != "ok" {
		t.Fatalf("unexpected toolResponse.status: %+v", resp)
	}
}
