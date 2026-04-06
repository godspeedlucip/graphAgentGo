package gateway

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	domain "go-sse-skeleton/internal/domain/gateway"
)

func TestAtomicRulesProviderUpdate(t *testing.T) {
	t.Parallel()

	p := NewAtomicRulesProvider(domain.Rules{DefaultTarget: domain.TargetJava})
	before, err := p.Current(context.Background())
	if err != nil {
		t.Fatalf("current before update: %v", err)
	}
	if before.DefaultTarget != domain.TargetJava {
		t.Fatalf("unexpected initial target: %s", before.DefaultTarget)
	}

	p.Update(domain.Rules{DefaultTarget: domain.TargetGo})
	after, err := p.Current(context.Background())
	if err != nil {
		t.Fatalf("current after update: %v", err)
	}
	if after.DefaultTarget != domain.TargetGo {
		t.Fatalf("unexpected updated target: %s", after.DefaultTarget)
	}
}

func TestFileRulesProviderHotReloadAndFailOpen(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "gateway_rules.json")
	if err := os.WriteFile(path, []byte(`{"defaultTarget":"go"}`), 0o644); err != nil {
		t.Fatalf("write initial rules: %v", err)
	}

	p, err := NewFileRulesProvider(path, domain.Rules{DefaultTarget: domain.TargetJava})
	if err != nil {
		t.Fatalf("new file rules provider: %v", err)
	}

	r1, err := p.Current(context.Background())
	if err != nil {
		t.Fatalf("current initial: %v", err)
	}
	if r1.DefaultTarget != domain.TargetGo {
		t.Fatalf("unexpected initial file target: %s", r1.DefaultTarget)
	}

	time.Sleep(5 * time.Millisecond)
	if err = os.WriteFile(path, []byte(`{"defaultTarget":"java"}`), 0o644); err != nil {
		t.Fatalf("write updated rules: %v", err)
	}
	r2, err := p.Current(context.Background())
	if err != nil {
		t.Fatalf("current after update: %v", err)
	}
	if r2.DefaultTarget != domain.TargetJava {
		t.Fatalf("unexpected updated file target: %s", r2.DefaultTarget)
	}

	time.Sleep(5 * time.Millisecond)
	if err = os.WriteFile(path, []byte(`{`), 0o644); err != nil {
		t.Fatalf("write broken rules: %v", err)
	}
	r3, err := p.Current(context.Background())
	if err != nil {
		t.Fatalf("current fail-open: %v", err)
	}
	if r3.DefaultTarget != domain.TargetJava {
		t.Fatalf("expected last known good target, got: %s", r3.DefaultTarget)
	}
}
