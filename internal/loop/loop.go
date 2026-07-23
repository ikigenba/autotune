// Package loop coordinates baseline measurement, prompt improvement, and
// final reporting for one tuning invocation.
package loop

import (
	"context"
	"fmt"
	"io"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/ikigenba/agentkit"
	"github.com/ikigenba/autotune/internal/config"
	"github.com/ikigenba/autotune/internal/folder"
	"github.com/ikigenba/autotune/internal/improver"
	"github.com/ikigenba/autotune/internal/runner"
	"github.com/ikigenba/autotune/internal/scorer"
	"github.com/ikigenba/autotune/internal/workspace"
)

const (
	ExitOK          = 0
	ExitFailure     = 1
	ExitBudget      = 3
	ExitInterrupted = 130
)

// Rails are the optional iteration, elapsed-time, and cost limits. Zero means
// unlimited.
type Rails struct {
	MaxIterations int
	MaxTime       time.Duration
	MaxSpend      float64
}

// Options controls the deterministic parts of a tuning run.
type Options struct {
	Repeat     int
	Parallel   int
	MaxRetries int
	WorstK     int
	Rails      Rails
}

// Outcome is the durable result of a tuning run.
type Outcome struct {
	Baseline   float64
	Epsilon    float64
	Best       float64
	Accepted   int
	Holdout    *float64
	Verdict    string
	StopReason string
}

// Store is the subset of workspace used by the tuning loop.
type Store interface {
	WriteConfigStamp(*config.Config) error
	WriteBaseline([]float64, float64, float64) error
	WriteCandidate(int, string, *runner.EvalResult) error
	PromoteBest(int) error
	AppendHistory(string) error
	WriteSummary(workspace.Summary, string) error
}

// Deps contains all effects owned outside the loop.
type Deps struct {
	RunnerConv   runner.NewConv
	ImproverConv runner.NewConv
	Scorer       scorer.Scorer
	Workspace    Store
	Now          func() time.Time
	Out          io.Writer
	Err          io.Writer
}

// Run executes one complete tuning invocation. All stop paths converge on
// finalize so the summary and final report are never skipped.
func Run(ctx context.Context, deps Deps, f *folder.Folder, cfg *config.Config, opts Options) (out Outcome, exit int) {
	opts = defaults(opts)
	if deps.Now == nil {
		deps.Now = time.Now
	}
	if deps.Out == nil {
		deps.Out = io.Discard
	}
	if deps.Err == nil {
		deps.Err = io.Discard
	}
	start := deps.Now()
	winner := ""
	totalCost := agentkit.Cost(0)
	var lastCases []runner.CaseResult
	var history []improver.Attempt
	warn := warningSink(deps.Err)

	fail := func(reason string, code int) {
		out.StopReason = reason
		exit = code
	}
	failErr := func(reason string, code int, err error) {
		fmt.Fprintln(deps.Err, err)
		fail(reason, code)
	}
	if f == nil || cfg == nil || deps.RunnerConv == nil || deps.ImproverConv == nil || deps.Scorer == nil || deps.Workspace == nil {
		fail("internal failure", ExitFailure)
		return finalize(ctx, deps, f, cfg, out, exit, winner, totalCost, warn)
	}
	if opts.Repeat < 1 || opts.Parallel < 1 || opts.Rails.MaxIterations < 0 || opts.Rails.MaxTime < 0 || opts.Rails.MaxSpend < 0 {
		fail("internal failure", ExitFailure)
		return finalize(ctx, deps, f, cfg, out, exit, winner, totalCost, warn)
	}
	if err := deps.Workspace.WriteConfigStamp(cfg); err != nil {
		failErr("internal failure", ExitFailure, err)
		return finalize(ctx, deps, f, cfg, out, exit, winner, totalCost, warn)
	}

	composites := make([]float64, 0, opts.Repeat)
	minimum, maximum := math.Inf(1), math.Inf(-1)
	for range opts.Repeat {
		ev, err := runner.Evaluate(ctx, deps.RunnerConv, deps.Scorer, f.Prompt, f.Dev, opts.Parallel, warn)
		if err != nil {
			failErr(contextStop(ctx, err), contextExit(ctx, err), err)
			return finalize(ctx, deps, f, cfg, out, exit, winner, totalCost, warn)
		}
		totalCost += ev.Cost
		composites = append(composites, ev.Composite)
		out.Baseline += ev.Composite
		minimum = min(minimum, ev.Composite)
		maximum = max(maximum, ev.Composite)
		lastCases = ev.Cases
	}
	out.Baseline /= float64(opts.Repeat)
	out.Epsilon = maximum - minimum
	out.Best = out.Baseline
	winner = f.Prompt
	if err := deps.Workspace.WriteBaseline(composites, out.Baseline, out.Epsilon); err != nil {
		failErr("internal failure", ExitFailure, err)
		return finalize(ctx, deps, f, cfg, out, exit, winner, totalCost, warn)
	}

	iterations := 0
	for {
		if opts.Rails.MaxIterations > 0 && iterations >= opts.Rails.MaxIterations {
			fail("max iterations", ExitBudget)
			break
		}
		if opts.Rails.MaxTime > 0 && deps.Now().Sub(start) >= opts.Rails.MaxTime {
			fail("max time", ExitBudget)
			break
		}
		if opts.Rails.MaxSpend > 0 && totalCost.USD() >= opts.Rails.MaxSpend {
			fail("max spend", ExitBudget)
			break
		}
		if out.Best == 1.0 {
			fail("perfect score", ExitOK)
			break
		}
		if err := ctx.Err(); err != nil {
			fail("interrupted", ExitInterrupted)
			break
		}

		iterations++
		evidence := improver.Evidence{
			Incumbent: winner,
			Baseline:  out.Baseline,
			Best:      out.Best,
			Epsilon:   out.Epsilon,
			Cases:     lastCases,
			History:   history,
			WorstK:    opts.WorstK,
		}
		tracked, improverCost := trackConversations(deps.ImproverConv)
		summary, candidate, err := improver.Propose(ctx, tracked, f.ImproveMD, evidence, opts.MaxRetries, warn)
		totalCost += improverCost()
		if err != nil {
			failErr(contextStop(ctx, err), contextExit(ctx, err), err)
			break
		}
		ev, err := runner.Evaluate(ctx, deps.RunnerConv, deps.Scorer, candidate, f.Dev, opts.Parallel, warn)
		if err != nil {
			failErr(contextStop(ctx, err), contextExit(ctx, err), err)
			break
		}
		totalCost += ev.Cost
		accepted := ev.Composite > out.Best+out.Epsilon
		attempt := improver.Attempt{Summary: summary, Composite: ev.Composite, Accepted: accepted}
		history = append(history, attempt)
		if err := deps.Workspace.WriteCandidate(iterations, candidate, ev); err != nil {
			failErr("internal failure", ExitFailure, err)
			break
		}
		if err := deps.Workspace.AppendHistory(historyLine(iterations, attempt)); err != nil {
			failErr("internal failure", ExitFailure, err)
			break
		}
		if accepted {
			if err := deps.Workspace.PromoteBest(iterations); err != nil {
				failErr("internal failure", ExitFailure, err)
				break
			}
			winner = candidate
			out.Best = ev.Composite
			out.Accepted++
			lastCases = ev.Cases
		}
	}

	return finalize(ctx, deps, f, cfg, out, exit, winner, totalCost, warn)
}

func defaults(opts Options) Options {
	if opts.Repeat == 0 {
		opts.Repeat = 3
	}
	if opts.Parallel == 0 {
		opts.Parallel = 1
	}
	if opts.MaxRetries == 0 {
		opts.MaxRetries = 2
	}
	if opts.WorstK == 0 {
		opts.WorstK = 3
	}
	return opts
}

func trackConversations(nc runner.NewConv) (runner.NewConv, func() agentkit.Cost) {
	var conversations []*agentkit.Conversation
	tracked := func(system string) (*agentkit.Conversation, error) {
		conversation, err := nc(system)
		if err == nil {
			conversations = append(conversations, conversation)
		}
		return conversation, err
	}
	cost := func() agentkit.Cost {
		var total agentkit.Cost
		for _, conversation := range conversations {
			total += conversation.TotalCost()
		}
		return total
	}
	return tracked, cost
}

func warningSink(w io.Writer) runner.WarnFunc {
	var mu sync.Mutex
	seen := make(map[agentkit.WarningCode]struct{})
	return func(warning agentkit.Warning) {
		mu.Lock()
		defer mu.Unlock()
		if _, exists := seen[warning.Code]; exists {
			return
		}
		seen[warning.Code] = struct{}{}
		fmt.Fprintf(w, "warning: %s — %s\n", warning.Setting, warning.Detail)
	}
}

func finalize(ctx context.Context, deps Deps, f *folder.Folder, cfg *config.Config, out Outcome, exit int, winner string, totalCost agentkit.Cost, warn runner.WarnFunc) (Outcome, int) {
	_ = cfg
	_ = totalCost
	if deps.Workspace == nil {
		return out, exit
	}
	if out.Accepted > 0 && f != nil && len(f.Holdout) > 0 {
		ev, err := runner.Evaluate(context.WithoutCancel(ctx), deps.RunnerConv, deps.Scorer, winner, f.Holdout, 1, warn)
		if err != nil {
			fmt.Fprintln(deps.Err, err)
			out.StopReason = "internal failure"
			exit = ExitFailure
		} else {
			holdout := ev.Composite
			out.Holdout = &holdout
			if holdout <= out.Baseline {
				out.Verdict = "OVERFIT"
			} else {
				out.Verdict = "generalized"
			}
		}
	}
	diff := ""
	if f != nil && out.Accepted > 0 {
		diff = unifiedDiff(f.Prompt, winner)
	}
	if err := deps.Workspace.WriteSummary(workspace.Summary{
		Baseline: out.Baseline, Epsilon: out.Epsilon, Best: out.Best,
		Accepted: out.Accepted, Holdout: out.Holdout, Verdict: out.Verdict, StopReason: out.StopReason,
	}, diff); err != nil {
		fmt.Fprintln(deps.Err, err)
		out.StopReason = "internal failure"
		exit = ExitFailure
	}
	printReport(deps.Out, out, diff)
	return out, exit
}

func contextStop(ctx context.Context, err error) string {
	if ctx.Err() != nil || err == context.Canceled || err == context.DeadlineExceeded {
		return "interrupted"
	}
	return "internal failure"
}

func contextExit(ctx context.Context, err error) int {
	if contextStop(ctx, err) == "interrupted" {
		return ExitInterrupted
	}
	return ExitFailure
}

func historyLine(iteration int, a improver.Attempt) string {
	return fmt.Sprintf("iteration=%d composite=%.6f accepted=%t summary=%q", iteration, a.Composite, a.Accepted, a.Summary)
}

func printReport(w io.Writer, out Outcome, diff string) {
	if out.Accepted == 0 {
		fmt.Fprintf(w, "no improvement found\nbaseline: %.6f\nepsilon: %.6f\nstop: %s\n", out.Baseline, out.Epsilon, out.StopReason)
		return
	}
	fmt.Fprintf(w, "accepted: %d\nbaseline: %.6f\nbest: %.6f\n", out.Accepted, out.Baseline, out.Best)
	if out.Holdout != nil {
		fmt.Fprintf(w, "holdout: %.6f\nverdict: %s\n", *out.Holdout, out.Verdict)
	}
	fmt.Fprint(w, diff)
}

func unifiedDiff(before, after string) string {
	if before == after {
		return ""
	}
	oldLines := splitLines(before)
	newLines := splitLines(after)
	var b strings.Builder
	fmt.Fprintf(&b, "--- prompt.txt\n+++ prompt.txt\n@@ -1,%d +1,%d @@\n", len(oldLines), len(newLines))
	for _, line := range oldLines {
		fmt.Fprintf(&b, "-%s\n", line)
	}
	for _, line := range newLines {
		fmt.Fprintf(&b, "+%s\n", line)
	}
	return b.String()
}

func splitLines(s string) []string {
	s = strings.TrimSuffix(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}
