package workspace

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ikigenba/autotune/internal/config"
	"github.com/ikigenba/autotune/internal/runner"
)

func TestCreateAndResolvedStateArtifacts(t *testing.T) {
	// R-RJP4-UI9I
	root := tuneFolder(t, "incumbent")
	now := time.Date(2026, 7, 22, 14, 15, 2, 0, time.FixedZone("local", -5*60*60))
	w, err := Create(root, now)
	if err != nil {
		t.Fatal(err)
	}
	wantDir := filepath.Join(root, "runs", "20260722-191502")
	if w.Dir != wantDir {
		t.Fatalf("Dir = %q, want %q", w.Dir, wantDir)
	}
	for _, dir := range []string{"candidates", "best"} {
		if info, err := os.Stat(filepath.Join(w.Dir, dir)); err != nil || !info.IsDir() {
			t.Fatalf("%s directory not created: %v", dir, err)
		}
	}
	cfg := &config.Config{
		Runner:   config.Section{Provider: "runner-provider", Model: "runner-model", MaxTokens: 123},
		Improver: config.Section{Provider: "improver-provider", Model: "improver-model", BaseURL: "https://example.invalid"},
	}
	if err := w.WriteConfigStamp(cfg); err != nil {
		t.Fatal(err)
	}
	if err := w.WriteBaseline([]float64{0.25, 0.5}, 0.375, 0.01); err != nil {
		t.Fatal(err)
	}
	var stamped config.Config
	readJSON(t, filepath.Join(w.Dir, "config.json"), &stamped)
	if stamped.Runner.Provider != cfg.Runner.Provider || stamped.Runner.MaxTokens != 123 || stamped.Improver.BaseURL != cfg.Improver.BaseURL {
		t.Fatalf("config stamp did not preserve resolved values: %#v", stamped)
	}
	baseline := read(t, filepath.Join(w.Dir, "baseline.json"))
	for _, want := range []string{"0.250000", "0.500000", "0.375000", "0.010000"} {
		if !bytes.Contains(baseline, []byte(want)) {
			t.Errorf("baseline artifact missing fixed value %s:\n%s", want, baseline)
		}
	}
}

func TestEveryCandidateHasDeterministicArtifacts(t *testing.T) {
	// R-RM4X-M1QW
	w := newWorkspace(t)
	ev := &runner.EvalResult{
		Cases:     []runner.CaseResult{{Score: 0.5}, {Score: 0.25}},
		Composite: 0.375,
	}
	if err := w.WriteCandidate(1, "accepted", ev); err != nil {
		t.Fatal(err)
	}
	first := read(t, filepath.Join(w.Dir, "candidates", "001-scorecard.json"))
	if err := w.WriteCandidate(2, "rejected", ev); err != nil {
		t.Fatal(err)
	}
	second := read(t, filepath.Join(w.Dir, "candidates", "002-scorecard.json"))
	if !bytes.Equal(first, second) {
		t.Fatalf("identical evaluations produced different scorecards:\n%s\n%s", first, second)
	}
	if got := string(read(t, filepath.Join(w.Dir, "candidates", "001-prompt.txt"))); got != "accepted" {
		t.Fatalf("accepted prompt = %q", got)
	}
	if got := string(read(t, filepath.Join(w.Dir, "candidates", "002-prompt.txt"))); got != "rejected" {
		t.Fatalf("rejected prompt = %q", got)
	}
	if err := w.WriteCandidate(2, "overwrite", ev); err == nil {
		t.Fatal("WriteCandidate overwrote an existing attempt")
	}
	if err := w.WriteCandidate(4, "gap", ev); err == nil {
		t.Fatal("WriteCandidate allowed a non-sequential attempt")
	}
	for _, want := range []string{"\"score\": 0.500000", "\"score\": 0.250000", "\"composite\": 0.375000"} {
		if !bytes.Contains(first, []byte(want)) {
			t.Errorf("scorecard missing %q:\n%s", want, first)
		}
	}
}

func TestPromoteBestOnlyOnAcceptance(t *testing.T) {
	// R-RNCT-ZTHL
	w := newWorkspace(t)
	accepted := &runner.EvalResult{Composite: 0.7}
	if err := w.WriteCandidate(1, "winner", accepted); err != nil {
		t.Fatal(err)
	}
	if err := w.PromoteBest(1); err != nil {
		t.Fatal(err)
	}
	wantPrompt := read(t, filepath.Join(w.Dir, "candidates", "001-prompt.txt"))
	wantScorecard := read(t, filepath.Join(w.Dir, "candidates", "001-scorecard.json"))
	bestPromptPath := filepath.Join(w.Dir, "best", "prompt.txt")
	bestScorecardPath := filepath.Join(w.Dir, "best", "scorecard.json")
	if !bytes.Equal(read(t, bestPromptPath), wantPrompt) || !bytes.Equal(read(t, bestScorecardPath), wantScorecard) {
		t.Fatal("promotion did not copy both candidate artifacts over best")
	}
	beforePrompt := read(t, bestPromptPath)
	beforeScorecard := read(t, bestScorecardPath)
	if err := w.WriteCandidate(2, "loser", &runner.EvalResult{Composite: 0.1}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(read(t, bestPromptPath), beforePrompt) || !bytes.Equal(read(t, bestScorecardPath), beforeScorecard) {
		t.Fatal("recording a rejected candidate changed best artifacts")
	}
}

func TestAppendHistoryAddsOneCompleteLinePerAttempt(t *testing.T) {
	// R-ROKQ-DL8A
	w := newWorkspace(t)
	lines := []string{
		"001 clarify instructions | 0.600000 | accept",
		"002 shorten examples | 0.550000 | reject",
	}
	for _, line := range lines {
		if err := w.AppendHistory(line); err != nil {
			t.Fatal(err)
		}
	}
	got := string(read(t, filepath.Join(w.Dir, "history.md")))
	if got != strings.Join(lines, "\n")+"\n" {
		t.Fatalf("history = %q", got)
	}
	if err := w.AppendHistory("003 invalid\nextra line"); err == nil {
		t.Fatal("AppendHistory accepted a multi-line attempt")
	}
}

func TestFullRunNeverWritesIncumbentAndSummarizesOutcome(t *testing.T) {
	// R-RPSM-RCYZ
	root := tuneFolder(t, "operator-owned prompt\n")
	before := read(t, filepath.Join(root, "prompt.txt"))
	w, err := Create(root, time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if err := w.WriteCandidate(1, "accepted candidate", &runner.EvalResult{Composite: 0.8}); err != nil {
		t.Fatal(err)
	}
	if err := w.PromoteBest(1); err != nil {
		t.Fatal(err)
	}
	if err := w.WriteCandidate(2, "rejected candidate", &runner.EvalResult{Composite: 0.7}); err != nil {
		t.Fatal(err)
	}
	holdout := 0.79
	summary := Summary{Baseline: 0.5, Epsilon: 0.01, Best: 0.8, Accepted: 1, Holdout: &holdout, Verdict: "pass", StopReason: "attempt limit"}
	if err := w.WriteSummary(summary, "-old\n+new\n"); err != nil {
		t.Fatal(err)
	}
	if after := read(t, filepath.Join(root, "prompt.txt")); !bytes.Equal(after, before) {
		t.Fatalf("incumbent changed from %q to %q", before, after)
	}
	artifact := string(read(t, filepath.Join(w.Dir, "summary.md")))
	for _, want := range []string{"Baseline: 0.500000", "Best: 0.800000", "Epsilon: 0.010000", "Holdout: 0.790000", "Verdict: pass", "Stop reason: attempt limit", "-old\n+new"} {
		if !strings.Contains(artifact, want) {
			t.Errorf("summary missing %q:\n%s", want, artifact)
		}
	}
}

func newWorkspace(t *testing.T) *Workspace {
	t.Helper()
	w, err := Create(tuneFolder(t, "incumbent"), time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return w
}

func tuneFolder(t *testing.T, prompt string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "prompt.txt"), []byte(prompt), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func read(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func readJSON(t *testing.T, path string, dst any) {
	t.Helper()
	if err := json.Unmarshal(read(t, path), dst); err != nil {
		t.Fatal(err)
	}
}
