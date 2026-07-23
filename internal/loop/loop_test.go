package loop

import (
	"bytes"
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ikigenba/agentkit"
	"github.com/ikigenba/autotune/internal/config"
	"github.com/ikigenba/autotune/internal/folder"
	"github.com/ikigenba/autotune/internal/runner"
	"github.com/ikigenba/autotune/internal/scorer"
	"github.com/ikigenba/autotune/internal/workspace"
)

func TestRunMeasuresBaselineAndNoiseFloor(t *testing.T) {
	// R-R52C-99D6
	h := newHarness([]reply{{text: "0.25"}, {text: "0.5"}, {text: "0.75"}, {text: "0.1"}}, []reply{proposal("worse", "candidate")})
	out, _ := h.run(context.Background(), Options{Repeat: 3, Rails: Rails{MaxIterations: 1}})
	if out.Baseline != 0.5 {
		t.Fatalf("baseline = %v, want 0.5", out.Baseline)
	}
	if out.Epsilon != 0.5 {
		t.Fatalf("epsilon = %v, want 0.5", out.Epsilon)
	}
	if got := h.store.baselineComposites; len(got) != 3 || got[0] != 0.25 || got[1] != 0.5 || got[2] != 0.75 {
		t.Fatalf("written composites = %v", got)
	}
}

func TestRunUsesStrictEpsilonAcceptanceAndUpdatesBest(t *testing.T) {
	// R-R7I5-0SUK
	h := newHarness(
		[]reply{{text: "0.25"}, {text: "0.5"}, {text: "0.625"}, {text: "0.75"}, {text: "1"}},
		[]reply{proposal("at threshold", "equal"), proposal("above threshold", "winner"), proposal("at new threshold", "later")},
	)
	h.folder.Holdout = nil
	out, code := h.run(context.Background(), Options{Repeat: 2, Rails: Rails{MaxIterations: 3}})
	if code != ExitBudget || out.Accepted != 1 || out.Best != 0.75 {
		t.Fatalf("outcome = %+v, code = %d", out, code)
	}
	if got := h.store.promoted; len(got) != 1 || got[0] != 2 {
		t.Fatalf("promoted candidates = %v, want [2]", got)
	}
	if len(h.store.candidates) != 3 || h.store.candidates[0].Composite != 0.625 || h.store.candidates[2].Composite != 1 {
		t.Fatalf("candidate evaluations = %+v", h.store.candidates)
	}
}

func TestRunStopsOnRailsInFixedOrderAndFinalizes(t *testing.T) {
	// R-R8Q1-EKL9
	t.Run("iterations", func(t *testing.T) {
		h := newHarness([]reply{{text: "0.5"}, {text: "0.4"}}, []reply{proposal("reject", "candidate")})
		out, code := h.run(context.Background(), Options{Repeat: 1, Rails: Rails{MaxIterations: 1}})
		assertRail(t, h, out, code, "max iterations")
	})
	t.Run("time", func(t *testing.T) {
		h := newHarness([]reply{{text: "0.5"}}, nil)
		start := time.Unix(100, 0)
		times := []time.Time{start, start.Add(2 * time.Second)}
		h.deps.Now = func() time.Time {
			n := times[0]
			times = times[1:]
			return n
		}
		out, code := h.run(context.Background(), Options{Repeat: 1, Rails: Rails{MaxTime: time.Second}})
		assertRail(t, h, out, code, "max time")
	})
	t.Run("spend", func(t *testing.T) {
		h := newHarness(
			[]reply{{text: "0.5", cost: agentkit.Cost(400_000_000)}, {text: "0.4", cost: agentkit.Cost(400_000_000)}},
			[]reply{{text: proposal("reject", "candidate").text, cost: agentkit.Cost(400_000_000)}},
		)
		out, code := h.run(context.Background(), Options{Repeat: 1, Rails: Rails{MaxSpend: 1}})
		assertRail(t, h, out, code, "max spend")
	})
	t.Run("order", func(t *testing.T) {
		h := newHarness([]reply{{text: "0.5"}, {text: "0.4", cost: agentkit.Cost(2_000_000_000)}}, []reply{proposal("reject", "candidate")})
		start := time.Unix(100, 0)
		calls := 0
		h.deps.Now = func() time.Time {
			calls++
			if calls < 3 {
				return start
			}
			return start.Add(2 * time.Second)
		}
		out, code := h.run(context.Background(), Options{Repeat: 1, Rails: Rails{MaxIterations: 1, MaxTime: time.Second, MaxSpend: 1}})
		assertRail(t, h, out, code, "max iterations")
	})
}

func TestRunStopsAtPerfectDevScore(t *testing.T) {
	// R-R9XX-SCBY
	h := newHarness([]reply{{text: "1"}}, nil)
	out, code := h.run(context.Background(), Options{Repeat: 1})
	if code != ExitOK || out.StopReason != "perfect score" || out.Best != 1 {
		t.Fatalf("outcome = %+v, code = %d", out, code)
	}
	if h.improver.callsCount() != 0 {
		t.Fatalf("improver calls = %d, want 0", h.improver.callsCount())
	}
}

func TestRunCancellationFinalizesAndWritesSummary(t *testing.T) {
	// R-RB5U-642N
	ctx, cancel := context.WithCancel(context.Background())
	h := newHarness([]reply{{text: "0.5"}}, []reply{{before: cancel, err: context.Canceled}})
	out, code := h.run(ctx, Options{Repeat: 1})
	if code != ExitInterrupted || out.StopReason != "interrupted" {
		t.Fatalf("outcome = %+v, code = %d", out, code)
	}
	if h.store.summary == nil || h.store.summary.StopReason != "interrupted" {
		t.Fatalf("summary = %+v", h.store.summary)
	}
}

func TestRunEvaluatesHoldoutOnceAndClassifiesVerdict(t *testing.T) {
	// R-RCDQ-JVTC
	for _, test := range []struct {
		name, score, verdict string
	}{
		{name: "equal is overfit", score: "0.5", verdict: "OVERFIT"},
		{name: "higher generalized", score: "0.6", verdict: "generalized"},
	} {
		t.Run(test.name, func(t *testing.T) {
			h := newHarness([]reply{{text: "0.5"}, {text: "0.8"}, {text: test.score}}, []reply{proposal("better", "winner")})
			out, code := h.run(context.Background(), Options{Repeat: 1, Rails: Rails{MaxIterations: 1}})
			if code != ExitBudget || out.Holdout == nil || out.Verdict != test.verdict {
				t.Fatalf("outcome = %+v, code = %d", out, code)
			}
			if h.runner.callsCount() != 3 {
				t.Fatalf("runner provider calls = %d, want baseline + candidate + one holdout", h.runner.callsCount())
			}
		})
	}
}

func TestRunWithNoAcceptsSkipsHoldoutAndReportsHonestly(t *testing.T) {
	// R-RDLM-XNK1
	h := newHarness([]reply{{text: "1"}}, nil)
	out, code := h.run(context.Background(), Options{Repeat: 1})
	if code != ExitOK || out.Accepted != 0 || out.Holdout != nil {
		t.Fatalf("outcome = %+v, code = %d", out, code)
	}
	if h.runner.callsCount() != 1 {
		t.Fatalf("runner provider calls = %d, holdout was unexpectedly evaluated", h.runner.callsCount())
	}
	if !bytes.Contains(h.output.Bytes(), []byte("no improvement found")) {
		t.Fatalf("report = %q", h.output.String())
	}
}

func TestRunWritesEveryFailureCauseToErr(t *testing.T) {
	// R-E50H-F4MC
	cause := errors.New("distinct failure cause")
	tests := []struct {
		name  string
		setup func(*harness)
	}{
		{
			name: "baseline evaluation",
			setup: func(h *harness) {
				h.runner.replies = []reply{{err: cause}}
			},
		},
		{
			name: "improver",
			setup: func(h *harness) {
				h.improver.replies = []reply{{err: cause}}
			},
		},
		{
			name: "candidate evaluation",
			setup: func(h *harness) {
				h.runner.replies = []reply{{text: "0.5"}, {err: cause}}
			},
		},
		{
			name: "workspace write",
			setup: func(h *harness) {
				h.store.writeCandidateErr = cause
			},
		},
		{
			name: "holdout evaluation",
			setup: func(h *harness) {
				h.runner.replies = []reply{{text: "0.5"}, {text: "0.8"}, {err: cause}}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			h := newHarness(
				[]reply{{text: "0.5"}, {text: "0.4"}},
				[]reply{proposal("candidate", "candidate")},
			)
			test.setup(h)
			out, code := h.run(context.Background(), Options{Repeat: 1, Rails: Rails{MaxIterations: 1}})
			if code != ExitFailure || out.StopReason != "internal failure" {
				t.Fatalf("outcome = %+v, code = %d", out, code)
			}
			if got := h.errOutput.String(); !strings.Contains(got, cause.Error()) {
				t.Fatalf("stderr = %q, want cause %q", got, cause)
			}
		})
	}
}

func TestRunAccumulatesComputedConversationCostsForSpendRail(t *testing.T) {
	// R-M3TR-MDZP
	pricing := agentkit.Pricing{Tiers: []agentkit.RateTier{{
		InputUncached: 10_000_000,
		Output:        20_000_000,
	}}}
	baselineUsage := agentkit.Usage{InputUncached: 5, Output: 2, Total: 7}
	improverUsage := agentkit.Usage{InputUncached: 7, Output: 3, Total: 10}
	candidateUsage := agentkit.Usage{InputUncached: 11, Output: 4, Total: 15}
	wantCost := pricing.Cost(baselineUsage) + pricing.Cost(improverUsage) + pricing.Cost(candidateUsage)
	maxSpend := wantCost.USD() - 0.000000001

	h := newHarness(
		[]reply{{text: "0.5", usage: baselineUsage}, {text: "0.4", usage: candidateUsage}},
		[]reply{{text: proposal("reject", "candidate").text, usage: improverUsage}},
	)
	h.deps.RunnerConv = newPricedConversation(h.runner, &pricing)
	h.deps.ImproverConv = newPricedConversation(h.improver, &pricing)
	out, code := h.run(context.Background(), Options{Repeat: 1, Rails: Rails{MaxSpend: maxSpend}})

	if code != ExitBudget || out.StopReason != "max spend" {
		t.Fatalf("outcome = %+v, code = %d, want computed-cost spend rail", out, code)
	}
	if h.runner.callsCount() != 2 || h.improver.callsCount() != 1 {
		t.Fatalf("calls = runner %d, improver %d, want all three priced turns before rail", h.runner.callsCount(), h.improver.callsCount())
	}
	if wantCost == 0 || maxSpend <= (pricing.Cost(baselineUsage)+pricing.Cost(improverUsage)).USD() {
		t.Fatalf("invalid cost boundary: exact total=%d max=%v", wantCost, maxSpend)
	}

	nextCall := errors.New("continued beyond exact first-iteration spend")
	h = newHarness(
		[]reply{{text: "0.5", usage: baselineUsage}, {text: "0.4", usage: candidateUsage}},
		[]reply{
			{text: proposal("reject", "candidate").text, usage: improverUsage},
			{err: nextCall},
		},
	)
	h.deps.RunnerConv = newPricedConversation(h.runner, &pricing)
	h.deps.ImproverConv = newPricedConversation(h.improver, &pricing)
	out, code = h.run(context.Background(), Options{Repeat: 1, Rails: Rails{MaxSpend: wantCost.USD() + 0.000000001}})
	if code != ExitFailure || out.StopReason != "internal failure" || !strings.Contains(h.errOutput.String(), nextCall.Error()) {
		t.Fatalf("above-exact boundary outcome = %+v, code = %d, stderr = %q", out, code, h.errOutput.String())
	}
	if h.improver.callsCount() != 2 {
		t.Fatalf("above-exact boundary improver calls = %d, want second iteration", h.improver.callsCount())
	}
}

func TestRunDeduplicatesAllProviderWarningCodesToErr(t *testing.T) {
	// R-M51O-05QE
	repeated := agentkit.Warning{Setting: "tool_choice", Code: agentkit.WarnToolChoiceForced, Detail: "forced choice degraded"}
	distinct := agentkit.Warning{Setting: "tool_schema", Code: agentkit.WarnToolSchemaLossy, Detail: "schema keywords removed"}
	h := newHarness(
		[]reply{
			{text: "0.5", warnings: []agentkit.Warning{repeated}},
			{text: "0.4", warnings: []agentkit.Warning{repeated, distinct}},
		},
		[]reply{{text: proposal("reject", "candidate").text, warnings: []agentkit.Warning{repeated}}},
	)

	_, _ = h.run(context.Background(), Options{Repeat: 1, Rails: Rails{MaxIterations: 1}})

	want := "warning: tool_choice — forced choice degraded\nwarning: tool_schema — schema keywords removed\n"
	if got := h.errOutput.String(); got != want {
		t.Fatalf("stderr = %q, want exactly %q", got, want)
	}
	if strings.Contains(h.output.String(), "warning:") {
		t.Fatalf("stdout contains warning line: %q", h.output.String())
	}
}

func TestRunSkipsUnparseableImproverReplyAndContinues(t *testing.T) {
	// R-SKPC-4XN1 — an improver reply that never parses skips its iteration
	// (recorded, counted against the rails) and the loop keeps going, ending
	// via a normal rail rather than aborting with an internal failure.
	h := newHarness(
		[]reply{{text: "0.5"}, {text: "0.4"}}, // baseline, then the second-iteration candidate
		[]reply{{text: "garbage"}, {text: "still garbage"}, proposal("second try", "candidate")},
	)
	out, code := h.run(context.Background(), Options{Repeat: 1, MaxRetries: 1, Rails: Rails{MaxIterations: 2}})

	if code != ExitBudget || out.StopReason != "max iterations" {
		t.Fatalf("outcome = %+v, code = %d; want the run to end via the rail, not abort", out, code)
	}
	if out.Skipped != 1 || out.Accepted != 0 {
		t.Fatalf("skipped = %d, accepted = %d; want one skip and no accepts", out.Skipped, out.Accepted)
	}
	if h.improver.callsCount() != 3 || h.runner.callsCount() != 2 {
		t.Fatalf("calls = improver %d, runner %d; want 2 failed + 1 good improver call and baseline + one candidate eval", h.improver.callsCount(), h.runner.callsCount())
	}
	if !containsLine(h.store.history, "skipped=true") {
		t.Fatalf("skip was not recorded in history: %v", h.store.history)
	}
	if !strings.Contains(h.errOutput.String(), "invalid after 2 attempts") {
		t.Fatalf("skip cause not surfaced on stderr: %q", h.errOutput.String())
	}
	if h.store.summary == nil || h.store.summary.Skipped != 1 {
		t.Fatalf("final summary did not record the skip: %+v", h.store.summary)
	}
}

func TestRunStopsWhenImproverIsPersistentlyUnusable(t *testing.T) {
	// R-SKPU-8QR2 — with no rails set, a run of consecutive unparseable replies
	// is genuinely unrecoverable and stops with ExitFailure, rather than
	// skipping forever and burning the budget.
	var improverReplies []reply
	for i := 0; i < 2*maxConsecutiveSkips; i++ {
		improverReplies = append(improverReplies, reply{text: "garbage"})
	}
	h := newHarness([]reply{{text: "0.5"}}, improverReplies)

	out, code := h.run(context.Background(), Options{Repeat: 1, MaxRetries: 1})

	if code != ExitFailure || out.StopReason != "improver unrecoverable" {
		t.Fatalf("outcome = %+v, code = %d; want unrecoverable stop", out, code)
	}
	if out.Skipped != maxConsecutiveSkips {
		t.Fatalf("skipped = %d, want %d", out.Skipped, maxConsecutiveSkips)
	}
	if h.improver.callsCount() != 2*maxConsecutiveSkips {
		t.Fatalf("improver calls = %d, want %d", h.improver.callsCount(), 2*maxConsecutiveSkips)
	}
	if !strings.Contains(h.errOutput.String(), "consecutive") {
		t.Fatalf("unrecoverable cause not surfaced on stderr: %q", h.errOutput.String())
	}
	if h.store.summary == nil || h.store.summary.StopReason != "improver unrecoverable" {
		t.Fatalf("finalize did not run for the unrecoverable stop: %+v", h.store.summary)
	}
}

func TestFinalReportAlwaysEndsWithStopLine(t *testing.T) {
	// R-FNST-6LM3 — the final report terminates with a `stop:` line on both the
	// accepted-winner and no-improvement paths; the winner diff is never the
	// last thing printed.
	t.Run("accepted winner", func(t *testing.T) {
		h := newHarness([]reply{{text: "0.5"}, {text: "1"}}, []reply{proposal("win", "winner")})
		h.folder.Holdout = nil
		out, code := h.run(context.Background(), Options{Repeat: 1, Rails: Rails{MaxIterations: 1}})
		if code != ExitBudget || out.Accepted != 1 {
			t.Fatalf("outcome = %+v, code = %d", out, code)
		}
		report := h.output.String()
		stop := strings.LastIndex(report, "stop: max iterations")
		diff := strings.Index(report, "--- prompt.txt")
		if stop < 0 || diff < 0 || stop < diff {
			t.Fatalf("report must print the diff then terminate with a stop line:\n%s", report)
		}
		if !strings.HasSuffix(strings.TrimRight(report, "\n"), "stop: max iterations") {
			t.Fatalf("report does not end with the stop line:\n%s", report)
		}
	})
	t.Run("no improvement", func(t *testing.T) {
		h := newHarness([]reply{{text: "1"}}, nil)
		h.run(context.Background(), Options{Repeat: 1})
		report := h.output.String()
		if !strings.Contains(report, "no improvement found") || !strings.Contains(report, "stop: perfect score") {
			t.Fatalf("no-improvement report missing stop line:\n%s", report)
		}
	})
}

func containsLine(lines []string, substr string) bool {
	for _, line := range lines {
		if strings.Contains(line, substr) {
			return true
		}
	}
	return false
}

type harness struct {
	deps      Deps
	folder    *folder.Folder
	store     *fakeStore
	runner    *scriptedProvider
	improver  *scriptedProvider
	output    *bytes.Buffer
	errOutput *bytes.Buffer
}

func newHarness(runnerReplies, improverReplies []reply) *harness {
	runnerProvider := &scriptedProvider{replies: runnerReplies}
	improverProvider := &scriptedProvider{replies: improverReplies}
	store := &fakeStore{}
	output := &bytes.Buffer{}
	errOutput := &bytes.Buffer{}
	return &harness{
		deps: Deps{
			RunnerConv:   newConversation(runnerProvider),
			ImproverConv: newConversation(improverProvider),
			Scorer:       numberScorer{},
			Workspace:    store,
			Now:          func() time.Time { return time.Unix(100, 0) },
			Out:          output,
			Err:          errOutput,
		},
		folder: &folder.Folder{
			Prompt:    "incumbent",
			ImproveMD: "improve it",
			Dev:       []folder.Case{{Name: "dev", Dir: "dev", Input: "input"}},
			Holdout:   []folder.Case{{Name: "holdout", Dir: "holdout", Input: "input"}},
		},
		store: store, runner: runnerProvider, improver: improverProvider, output: output, errOutput: errOutput,
	}
}

func (h *harness) run(ctx context.Context, opts Options) (Outcome, int) {
	return Run(ctx, h.deps, h.folder, &config.Config{}, opts)
}

func newConversation(provider agentkit.Provider) runner.NewConv {
	pricing := &agentkit.Pricing{}
	return newPricedConversation(provider, pricing)
}

func newPricedConversation(provider agentkit.Provider, pricing *agentkit.Pricing) runner.NewConv {
	return func(system string) (*agentkit.Conversation, error) {
		return &agentkit.Conversation{Provider: provider, Model: "test", System: system, Pricing: pricing}, nil
	}
}

type reply struct {
	text     string
	cost     agentkit.Cost
	usage    agentkit.Usage
	warnings []agentkit.Warning
	err      error
	before   func()
}

func proposal(summary, prompt string) reply {
	return reply{text: "SUMMARY: " + summary + "\n\n```\n" + prompt + "\n```"}
}

type scriptedProvider struct {
	mu      sync.Mutex
	replies []reply
	calls   int
}

func (p *scriptedProvider) Name() string { return "scripted" }

func (p *scriptedProvider) RoundTrip(context.Context, *agentkit.Request) *agentkit.RoundTrip {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls++
	if len(p.replies) == 0 {
		return agentkit.NewRoundTrip(agentkit.Message{}, agentkit.FinishOther, agentkit.Usage{}, nil, errors.New("script exhausted"), 0, false)
	}
	r := p.replies[0]
	p.replies = p.replies[1:]
	if r.before != nil {
		r.before()
	}
	message := agentkit.Message{Role: agentkit.RoleAssistant, Blocks: []agentkit.Block{agentkit.TextBlock{Text: r.text}}}
	return agentkit.NewRoundTrip(message, agentkit.FinishStop, r.usage, r.warnings, r.err, r.cost, r.cost != 0)
}

func (p *scriptedProvider) callsCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.calls
}

type numberScorer struct{}

func (numberScorer) Score(_ context.Context, _ string, output string) (scorer.Result, error) {
	n, err := strconv.ParseFloat(output, 64)
	return scorer.Result{Score: n}, err
}

type fakeStore struct {
	baselineComposites []float64
	candidates         []*runner.EvalResult
	promoted           []int
	history            []string
	summary            *workspace.Summary
	diff               string
	writeCandidateErr  error
}

func (s *fakeStore) WriteConfigStamp(*config.Config) error { return nil }
func (s *fakeStore) WriteBaseline(composites []float64, _, _ float64) error {
	s.baselineComposites = append([]float64(nil), composites...)
	return nil
}
func (s *fakeStore) WriteCandidate(_ int, _ string, ev *runner.EvalResult) error {
	if s.writeCandidateErr != nil {
		return s.writeCandidateErr
	}
	s.candidates = append(s.candidates, ev)
	return nil
}
func (s *fakeStore) PromoteBest(n int) error {
	s.promoted = append(s.promoted, n)
	return nil
}
func (s *fakeStore) AppendHistory(line string) error {
	s.history = append(s.history, line)
	return nil
}
func (s *fakeStore) WriteSummary(summary workspace.Summary, diff string) error {
	s.summary = &summary
	s.diff = diff
	return nil
}

func assertRail(t *testing.T, h *harness, out Outcome, code int, reason string) {
	t.Helper()
	if code != ExitBudget || out.StopReason != reason {
		t.Fatalf("outcome = %+v, code = %d; want rail %q", out, code, reason)
	}
	if h.store.summary == nil || h.store.summary.StopReason != reason {
		t.Fatalf("final summary = %+v", h.store.summary)
	}
}
