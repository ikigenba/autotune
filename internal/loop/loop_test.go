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
	return func(system string) (*agentkit.Conversation, error) {
		return &agentkit.Conversation{Provider: provider, Model: "test", System: system}, nil
	}
}

type reply struct {
	text   string
	cost   agentkit.Cost
	err    error
	before func()
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
	return agentkit.NewRoundTrip(message, agentkit.FinishStop, agentkit.Usage{}, nil, r.err, r.cost, r.cost != 0)
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
func (s *fakeStore) AppendHistory(string) error { return nil }
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
