package app

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ikigenba/agentkit"
	"github.com/ikigenba/autotune/internal/config"
)

type scriptedProvider struct {
	mu      sync.Mutex
	outputs []string
	index   int
}

func (p *scriptedProvider) Name() string { return "scripted" }

func (p *scriptedProvider) RoundTrip(_ context.Context, req *agentkit.Request) *agentkit.RoundTrip {
	p.mu.Lock()
	defer p.mu.Unlock()
	text := "bad"
	if p.index < len(p.outputs) {
		text = p.outputs[p.index]
	} else if strings.Contains(req.System, "good") {
		text = "good"
	}
	p.index++
	return agentkit.NewRoundTrip(
		agentkit.Message{Role: agentkit.RoleAssistant, Blocks: []agentkit.Block{agentkit.TextBlock{Text: text}}},
		agentkit.FinishStop, agentkit.Usage{InputUncached: 1, Output: 1, Total: 2}, nil, nil, agentkit.Cost(1), true,
	)
}

type e2eResult struct {
	root, stdout, stderr string
	code                 int
}

func runEndToEnd(t *testing.T, tty bool, noColor bool) e2eResult {
	t.Helper()
	root := filepath.Join(t.TempDir(), "tune")
	var stdout, stderr bytes.Buffer
	getenv := func(string) string { return "" }
	if noColor {
		getenv = func(key string) string {
			if key == "NO_COLOR" {
				return "1"
			}
			return ""
		}
	}
	deps := Deps{
		Stdout: &stdout, Stderr: &stderr, Getenv: getenv,
		Now:  func() time.Time { return time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC) },
		Home: t.TempDir(), IsTTY: tty,
	}
	if code := Run(context.Background(), deps, []string{"--init", root}); code != 0 {
		t.Fatalf("init exit = %d, stderr = %q", code, stderr.String())
	}
	write := func(name, body string, mode os.FileMode) {
		t.Helper()
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), mode); err != nil {
			t.Fatal(err)
		}
	}
	write("prompt.txt", "bad\n", 0o644)
	write("improve.md", "Return a better prompt.\n", 0o644)
	write("cases/dev/example/input.txt", "dev input\n", 0o644)
	write("cases/holdout/one/input.txt", "holdout input\n", 0o644)
	write("score", "#!/bin/sh\ninput=$(cat)\ncase \"$input\" in *perfect*) score=1.0;; *good*) score=0.8;; *bad*) score=0.4;; *) score=0.5;; esac\nprintf '{\"score\":%s,\"feedback\":\"fixture\"}\\n' \"$score\"\n", 0o755)

	runner := &scriptedProvider{outputs: []string{"baseline", "baseline", "good", "bad", "perfect", "perfect"}}
	improver := &scriptedProvider{outputs: []string{
		"SUMMARY: make it good\n```\ngood\n```",
		"SUMMARY: make it bad\n```\nbad\n```",
		"SUMMARY: make it perfect\n```\nperfect\n```",
	}}
	deps.NewProvider = func(section config.Section, system string) (*agentkit.Conversation, error) {
		provider := agentkit.Provider(runner)
		if strings.Contains(section.Model, "sol") {
			provider = improver
		}
		return &agentkit.Conversation{Provider: provider, Model: "test", System: system}, nil
	}
	code := Run(context.Background(), deps, []string{"--repeat=2", "--parallel=1", root})
	return e2eResult{root: root, stdout: stdout.String(), stderr: stderr.String(), code: code}
}

// R-RX41-1ZF5
func TestRunEndToEndPersistsAcceptedRejectedAndFinalizedRun(t *testing.T) {
	r := runEndToEnd(t, false, false)
	if r.code != 0 {
		t.Fatalf("exit = %d, stderr = %q, stdout = %q", r.code, r.stderr, r.stdout)
	}
	runs, err := filepath.Glob(filepath.Join(r.root, "runs", "*"))
	if err != nil || len(runs) != 1 {
		t.Fatalf("runs = %v, err = %v", runs, err)
	}
	for _, name := range []string{"config.json", "baseline.json", "best", "history.md", "summary.md"} {
		if _, err := os.Stat(filepath.Join(runs[0], name)); err != nil {
			t.Errorf("missing %s: %v", name, err)
		}
	}
	candidates, err := filepath.Glob(filepath.Join(runs[0], "candidates", "*"))
	if err != nil || len(candidates) != 6 {
		t.Errorf("candidates = %v, err = %v", candidates, err)
	}
	summary, err := os.ReadFile(filepath.Join(runs[0], "summary.md"))
	if err != nil || !strings.Contains(string(summary), "generalized") {
		t.Fatalf("summary = %q, err = %v", summary, err)
	}
	history, err := os.ReadFile(filepath.Join(runs[0], "history.md"))
	if err != nil || !strings.Contains(string(history), "accepted=true") || !strings.Contains(string(history), "accepted=false") {
		t.Fatalf("history = %q, err = %v", history, err)
	}
}

// R-RUO8-AFXR
func TestRunPipedHealthLogDescribesTheFullRun(t *testing.T) {
	r := runEndToEnd(t, false, false)
	if r.code != 0 {
		t.Fatalf("exit = %d, stderr = %q", r.code, r.stderr)
	}
	for _, want := range []string{"baseline", "epsilon", "1/2", "2/2", "ACCEPT", "reject", "spend", "generalized", "---", "+++"} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("stdout missing %q:\n%s", want, r.stdout)
		}
	}
}

// R-RVW4-O7OG
func TestRunTerminalRenderingHonorsTTYAndNoColor(t *testing.T) {
	plain := runEndToEnd(t, false, false)
	noColor := runEndToEnd(t, true, true)
	tty := runEndToEnd(t, true, false)
	if plain.code != 0 || noColor.code != 0 || tty.code != 0 {
		t.Fatalf("run codes: plain=%d no-color=%d tty=%d", plain.code, noColor.code, tty.code)
	}
	if strings.Contains(plain.stdout, "\x1b[") || strings.Contains(noColor.stdout, "\x1b[") {
		t.Fatalf("ANSI escaped non-color output: plain=%q no-color=%q", plain.stdout, noColor.stdout)
	}
	if strings.Contains(plain.stdout, "\r") || !strings.Contains(tty.stdout, "\r") {
		t.Fatalf("ticker rendering mismatch: plain=%q tty=%q", plain.stdout, tty.stdout)
	}
}
