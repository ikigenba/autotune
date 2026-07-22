// Package workspace manages the durable artifacts produced by one tuning run.
package workspace

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ikigenba/autotune/internal/config"
	"github.com/ikigenba/autotune/internal/runner"
)

const runIDFormat = "20060102-150405"

// Workspace is the durable record for one invocation.
type Workspace struct {
	Dir string
}

// Summary contains the final outcome of a run.
type Summary struct {
	Baseline, Epsilon, Best float64
	Accepted                int
	Holdout                 *float64
	Verdict, StopReason     string
}

// Create creates a new timestamped run directory and seeds its best prompt
// from the tune folder's incumbent. An existing run is never resumed.
func Create(folderRoot string, now time.Time) (*Workspace, error) {
	if folderRoot == "" {
		return nil, errors.New("workspace: folder root is empty")
	}

	incumbent, err := os.ReadFile(filepath.Join(folderRoot, "prompt.txt"))
	if err != nil {
		return nil, fmt.Errorf("workspace: read incumbent prompt: %w", err)
	}

	runsDir := filepath.Join(folderRoot, "runs")
	if err := os.MkdirAll(runsDir, 0o755); err != nil {
		return nil, fmt.Errorf("workspace: create runs directory: %w", err)
	}
	dir := filepath.Join(runsDir, now.UTC().Format(runIDFormat))
	if err := os.Mkdir(dir, 0o755); err != nil {
		return nil, fmt.Errorf("workspace: create run directory: %w", err)
	}

	w := &Workspace{Dir: dir}
	if err := w.initialize(incumbent); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *Workspace) initialize(incumbent []byte) error {
	for _, name := range []string{"candidates", "best"} {
		if err := os.Mkdir(filepath.Join(w.Dir, name), 0o755); err != nil {
			return fmt.Errorf("workspace: create %s directory: %w", name, err)
		}
	}
	if err := writeFile(filepath.Join(w.Dir, "best", "prompt.txt"), incumbent); err != nil {
		return fmt.Errorf("workspace: seed best prompt: %w", err)
	}
	if err := writeFile(filepath.Join(w.Dir, "history.md"), nil); err != nil {
		return fmt.Errorf("workspace: create history: %w", err)
	}
	return nil
}

// WriteConfigStamp records the fully resolved runner and improver config.
func (w *Workspace) WriteConfigStamp(cfg *config.Config) error {
	if cfg == nil {
		return errors.New("workspace: config is nil")
	}
	stamp := struct {
		Runner   config.Section `json:"runner"`
		Improver config.Section `json:"improver"`
	}{Runner: cfg.Runner, Improver: cfg.Improver}
	b, err := json.MarshalIndent(stamp, "", "  ")
	if err != nil {
		return fmt.Errorf("workspace: encode config: %w", err)
	}
	b = append(b, '\n')
	if err := writeFile(filepath.Join(w.Dir, "config.json"), b); err != nil {
		return fmt.Errorf("workspace: write config: %w", err)
	}
	return nil
}

// WriteBaseline records every baseline repeat and its aggregate values.
func (w *Workspace) WriteBaseline(composites []float64, baseline, epsilon float64) error {
	for i, score := range composites {
		if !finite(score) {
			return fmt.Errorf("workspace: baseline composite %d is not finite", i)
		}
	}
	if !finite(baseline) || !finite(epsilon) {
		return errors.New("workspace: baseline and epsilon must be finite")
	}
	var b bytes.Buffer
	b.WriteString("{\n  \"composites\": [")
	for i, score := range composites {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(fixedFloat(score))
	}
	b.WriteString("],\n  \"baseline\": ")
	b.WriteString(fixedFloat(baseline))
	b.WriteString(",\n  \"epsilon\": ")
	b.WriteString(fixedFloat(epsilon))
	b.WriteString("\n}\n")
	if err := writeFile(filepath.Join(w.Dir, "baseline.json"), b.Bytes()); err != nil {
		return fmt.Errorf("workspace: write baseline: %w", err)
	}
	return nil
}

// WriteCandidate records an attempted prompt and its scorecard.
func (w *Workspace) WriteCandidate(n int, prompt string, ev *runner.EvalResult) error {
	if n < 1 || n > 999 {
		return fmt.Errorf("workspace: candidate number %d is outside 1..999", n)
	}
	if ev == nil {
		return errors.New("workspace: candidate evaluation is nil")
	}
	if !finite(ev.Composite) {
		return errors.New("workspace: candidate composite is not finite")
	}
	for i, result := range ev.Cases {
		if !finite(result.Score) {
			return fmt.Errorf("workspace: candidate case score %d is not finite", i)
		}
	}
	if err := w.requireNextCandidate(n); err != nil {
		return err
	}
	prefix := filepath.Join(w.Dir, "candidates", fmt.Sprintf("%03d", n))
	if err := writeFile(prefix+"-prompt.txt", []byte(prompt)); err != nil {
		return fmt.Errorf("workspace: write candidate prompt: %w", err)
	}
	if err := writeScorecard(prefix+"-scorecard.json", ev); err != nil {
		return fmt.Errorf("workspace: write candidate scorecard: %w", err)
	}
	return nil
}

func (w *Workspace) requireNextCandidate(n int) error {
	candidatesDir := filepath.Join(w.Dir, "candidates")
	current := filepath.Join(candidatesDir, fmt.Sprintf("%03d-prompt.txt", n))
	if _, err := os.Stat(current); err == nil {
		return fmt.Errorf("workspace: candidate %03d already exists", n)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("workspace: inspect candidate %03d: %w", n, err)
	}
	if n == 1 {
		return nil
	}
	previous := filepath.Join(candidatesDir, fmt.Sprintf("%03d-scorecard.json", n-1))
	if _, err := os.Stat(previous); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("workspace: candidate %03d is not sequential", n)
		}
		return fmt.Errorf("workspace: inspect prior candidate %03d: %w", n-1, err)
	}
	return nil
}

// PromoteBest replaces the best artifacts with the named candidate.
func (w *Workspace) PromoteBest(n int) error {
	if n < 1 || n > 999 {
		return fmt.Errorf("workspace: candidate number %d is outside 1..999", n)
	}
	prefix := filepath.Join(w.Dir, "candidates", fmt.Sprintf("%03d", n))
	artifacts := []struct {
		sourceSuffix string
		destination  string
		contents     []byte
	}{
		{sourceSuffix: "-prompt.txt", destination: "prompt.txt"},
		{sourceSuffix: "-scorecard.json", destination: "scorecard.json"},
	}
	for i := range artifacts {
		b, err := os.ReadFile(prefix + artifacts[i].sourceSuffix)
		if err != nil {
			return fmt.Errorf("workspace: read candidate artifact: %w", err)
		}
		artifacts[i].contents = b
	}
	for _, artifact := range artifacts {
		if err := writeFile(filepath.Join(w.Dir, "best", artifact.destination), artifact.contents); err != nil {
			return fmt.Errorf("workspace: promote candidate artifact: %w", err)
		}
	}
	return nil
}

// AppendHistory appends exactly one line to the run history.
func (w *Workspace) AppendHistory(line string) error {
	if line == "" {
		return errors.New("workspace: history line is empty")
	}
	if strings.ContainsAny(line, "\r\n") {
		return errors.New("workspace: history entry must be exactly one line")
	}
	f, err := os.OpenFile(filepath.Join(w.Dir, "history.md"), os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		return fmt.Errorf("workspace: open history: %w", err)
	}
	if _, err := f.WriteString(line + "\n"); err != nil {
		_ = f.Close()
		return fmt.Errorf("workspace: append history: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("workspace: close history: %w", err)
	}
	return nil
}

// WriteSummary records the final run outcome and the operator-adoptable diff.
func (w *Workspace) WriteSummary(s Summary, diff string) error {
	if !finite(s.Baseline) || !finite(s.Epsilon) || !finite(s.Best) || (s.Holdout != nil && !finite(*s.Holdout)) {
		return errors.New("workspace: summary scores must be finite")
	}
	if s.Accepted < 0 {
		return errors.New("workspace: accepted count is negative")
	}
	var b strings.Builder
	b.WriteString("# Run summary\n\n")
	fmt.Fprintf(&b, "- Baseline: %s\n", fixedFloat(s.Baseline))
	fmt.Fprintf(&b, "- Best: %s\n", fixedFloat(s.Best))
	fmt.Fprintf(&b, "- Epsilon: %s\n", fixedFloat(s.Epsilon))
	fmt.Fprintf(&b, "- Accepted: %d\n", s.Accepted)
	if s.Holdout == nil {
		b.WriteString("- Holdout: not run\n")
	} else {
		fmt.Fprintf(&b, "- Holdout: %s\n", fixedFloat(*s.Holdout))
	}
	fmt.Fprintf(&b, "- Verdict: %s\n", s.Verdict)
	fmt.Fprintf(&b, "- Stop reason: %s\n", s.StopReason)
	b.WriteString("\n## Diff\n\n```diff\n")
	b.WriteString(diff)
	if diff != "" && !strings.HasSuffix(diff, "\n") {
		b.WriteByte('\n')
	}
	b.WriteString("```\n")
	if err := writeFile(filepath.Join(w.Dir, "summary.md"), []byte(b.String())); err != nil {
		return fmt.Errorf("workspace: write summary: %w", err)
	}
	return nil
}

func writeScorecard(path string, ev *runner.EvalResult) error {
	var b bytes.Buffer
	b.WriteString("{\n  \"cases\": [")
	for i, result := range ev.Cases {
		if i > 0 {
			b.WriteByte(',')
		}
		encodedCase, err := json.Marshal(result.Case)
		if err != nil {
			return err
		}
		b.WriteString("\n    {\"case\": ")
		b.Write(encodedCase)
		b.WriteString(", \"score\": ")
		b.WriteString(fixedFloat(result.Score))
		b.WriteByte('}')
	}
	if len(ev.Cases) > 0 {
		b.WriteByte('\n')
	}
	b.WriteString("  ],\n  \"composite\": ")
	b.WriteString(fixedFloat(ev.Composite))
	b.WriteString("\n}\n")
	return writeFile(path, b.Bytes())
}

func fixedFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', 6, 64)
}

func finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func writeFile(path string, contents []byte) error {
	return os.WriteFile(path, contents, 0o644)
}
