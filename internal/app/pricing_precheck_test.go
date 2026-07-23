package app

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ikigenba/agentkit"
	"github.com/ikigenba/autotune/internal/config"
)

func TestRunRejectsUnpricedSpendRailBeforeProviderCall(t *testing.T) {
	root := filepath.Join(t.TempDir(), "tune")
	var stdout, stderr bytes.Buffer
	providerCalls := 0
	deps := Deps{
		Stdout: &stdout,
		Stderr: &stderr,
		Getenv: func(string) string { return "" },
		Now:    func() time.Time { return time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC) },
		Home:   t.TempDir(),
		NewProvider: func(config.Section, string) (*agentkit.Conversation, error) {
			providerCalls++
			return nil, nil
		},
	}
	if code := Run(context.Background(), deps, []string{"--init", root}); code != 0 {
		t.Fatalf("init exit = %d, stderr = %q", code, stderr.String())
	}
	raw := []byte(`{
  "runner": {"provider":"openai","model":"runner-private","auth":"key"},
  "improver": {"provider":"openai","model":"gpt-5.6-luna","auth":"key"}
}`)
	if err := os.WriteFile(filepath.Join(root, "config.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}

	stderr.Reset()
	code := Run(context.Background(), deps, []string{"--max-spend=1", root})
	if code != 2 {
		t.Fatalf("exit = %d, want usage exit 2; stderr = %q", code, stderr.String())
	}
	if providerCalls != 0 {
		t.Fatalf("provider calls = %d, want 0", providerCalls)
	}
	if !strings.Contains(stderr.String(), "runner") || !strings.Contains(stderr.String(), "runner-private") {
		t.Fatalf("stderr = %q, want section and model", stderr.String())
	}
}
