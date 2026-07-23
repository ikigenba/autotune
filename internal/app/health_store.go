package app

import (
	"fmt"
	"io"
	"strings"

	"github.com/ikigenba/agentkit"
	"github.com/ikigenba/autotune/internal/config"
	"github.com/ikigenba/autotune/internal/runner"
	"github.com/ikigenba/autotune/internal/workspace"
)

type healthStore struct {
	workspace *workspace.Workspace
	out       io.Writer
	tty       bool
	color     bool
	spend     agentkit.Cost
}

func (s *healthStore) WriteConfigStamp(cfg *config.Config) error {
	return s.workspace.WriteConfigStamp(cfg)
}

func (s *healthStore) WriteBaseline(composites []float64, baseline, epsilon float64) error {
	if err := s.workspace.WriteBaseline(composites, baseline, epsilon); err != nil {
		return err
	}
	for i, composite := range composites {
		fmt.Fprintf(s.out, "baseline repeat %d/%d composite=%.6f\n", i+1, len(composites), composite)
	}
	fmt.Fprintf(s.out, "baseline=%.6f epsilon=%.6f repeats=%d\n", baseline, epsilon, len(composites))
	return nil
}

func (s *healthStore) WriteCandidate(n int, prompt string, ev *runner.EvalResult) error {
	if err := s.workspace.WriteCandidate(n, prompt, ev); err != nil {
		return err
	}
	s.spend += ev.Cost
	if s.tty {
		fmt.Fprintf(s.out, "\revaluating %d/%d", n, n)
	} else {
		fmt.Fprintf(s.out, "evaluating %d/%d\n", n, n)
	}
	return nil
}

func (s *healthStore) PromoteBest(n int) error { return s.workspace.PromoteBest(n) }

func (s *healthStore) AppendHistory(line string) error {
	if err := s.workspace.AppendHistory(line); err != nil {
		return err
	}
	verdict := "reject"
	switch {
	case strings.Contains(line, "skipped=true"):
		verdict = "skip"
	case strings.Contains(line, "accepted=true"):
		verdict = "ACCEPT"
	}
	if s.color {
		switch verdict {
		case "ACCEPT":
			verdict = "\x1b[32mACCEPT\x1b[0m"
		case "skip":
			verdict = "\x1b[33mskip\x1b[0m"
		default:
			verdict = "\x1b[31mreject\x1b[0m"
		}
	}
	if s.tty {
		fmt.Fprintln(s.out)
	}
	fmt.Fprintf(s.out, "%s verdict=%s cumulative-spend=$%.6f\n", strings.TrimSpace(line), verdict, s.spend.USD())
	return nil
}

func (s *healthStore) WriteSummary(summary workspace.Summary, diff string) error {
	if err := s.workspace.WriteSummary(summary, diff); err != nil {
		return err
	}
	if summary.Holdout != nil {
		fmt.Fprintf(s.out, "finalize holdout=%.6f verdict=%s\n", *summary.Holdout, summary.Verdict)
	} else {
		fmt.Fprintf(s.out, "finalize verdict=%s\n", summary.Verdict)
	}
	if diff != "" {
		fmt.Fprintln(s.out, "winner: best/prompt.txt")
		fmt.Fprint(s.out, diff)
		if !strings.HasSuffix(diff, "\n") {
			fmt.Fprintln(s.out)
		}
	}
	return nil
}
