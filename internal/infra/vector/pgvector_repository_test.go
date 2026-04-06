package vector

import (
	"strings"
	"testing"
)

func TestBuildKeywordSearchAllKBQueryCaseSumWeighting(t *testing.T) {
	t.Parallel()

	query, args, empty := buildKeywordSearchAllKBQuery([]string{"alpha", "beta"}, 5)
	if empty {
		t.Fatal("expected non-empty query")
	}
	if len(args) != 3 {
		t.Fatalf("unexpected args len: %d", len(args))
	}
	if args[0] != "%alpha%" || args[1] != "%beta%" || args[2] != 5 {
		t.Fatalf("unexpected args: %+v", args)
	}
	if !strings.Contains(query, "CASE WHEN lower(content) LIKE lower($1) THEN 1 ELSE 0 END") {
		t.Fatalf("query missing first CASE weighting: %s", query)
	}
	if !strings.Contains(query, "CASE WHEN lower(content) LIKE lower($2) THEN 1 ELSE 0 END") {
		t.Fatalf("query missing second CASE weighting: %s", query)
	}
	if !strings.Contains(query, "ORDER BY (") || !strings.Contains(query, ") DESC, updated_at DESC") {
		t.Fatalf("query missing expected ORDER BY weighting+updated_at: %s", query)
	}
}

func TestBuildKeywordSearchAllKBQueryIgnoresBlankKeywords(t *testing.T) {
	t.Parallel()

	_, _, empty := buildKeywordSearchAllKBQuery([]string{" ", "\t"}, 3)
	if !empty {
		t.Fatal("expected empty=true when all keywords are blank")
	}
}
