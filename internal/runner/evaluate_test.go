package runner

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/ikigenba/agentkit"
	"github.com/ikigenba/autotune/internal/folder"
	"github.com/ikigenba/autotune/internal/scorer"
)

type providerFunc func(context.Context, *agentkit.Request) *agentkit.RoundTrip

func (f providerFunc) RoundTrip(ctx context.Context, req *agentkit.Request) *agentkit.RoundTrip {
	return f(ctx, req)
}

func (providerFunc) Name() string { return "scripted" }

type scorerFunc func(context.Context, string, string) (scorer.Result, error)

func (f scorerFunc) Score(ctx context.Context, caseDir, output string) (scorer.Result, error) {
	return f(ctx, caseDir, output)
}

func successfulRoundTrip(output string, cost agentkit.Cost) *agentkit.RoundTrip {
	return agentkit.NewRoundTrip(
		agentkit.Message{Role: agentkit.RoleAssistant, Blocks: []agentkit.Block{agentkit.TextBlock{Text: output}}},
		agentkit.FinishStop,
		agentkit.Usage{},
		nil,
		nil,
		cost,
		true,
	)
}

func failedRoundTrip(err error) *agentkit.RoundTrip {
	return agentkit.NewRoundTrip(agentkit.Message{}, agentkit.FinishOther, agentkit.Usage{}, nil, err, 0, false)
}

func requestText(req *agentkit.Request) string {
	if len(req.Messages) == 0 {
		return ""
	}
	var text string
	for _, block := range req.Messages[len(req.Messages)-1].Blocks {
		if block, ok := block.(agentkit.TextBlock); ok {
			text += block.Text
		}
	}
	return text
}

// R-QVB5-73FM
func TestEvaluateUsesFreshBareConversationAndCapturesFinalAssistantText(t *testing.T) {
	var mu sync.Mutex
	var conversations []*agentkit.Conversation
	var systems, inputs []string
	var toolCounts []int
	nc := func(system string) (*agentkit.Conversation, error) {
		provider := providerFunc(func(_ context.Context, req *agentkit.Request) *agentkit.RoundTrip {
			mu.Lock()
			systems = append(systems, req.System)
			inputs = append(inputs, requestText(req))
			toolCounts = append(toolCounts, len(req.Tools))
			mu.Unlock()
			return successfulRoundTrip("answer:"+requestText(req), 1)
		})
		conv := &agentkit.Conversation{
			Provider: provider,
			Model:    "test-model",
			System:   "wrong",
			History: []agentkit.Message{{
				Role:   agentkit.RoleUser,
				Blocks: []agentkit.Block{agentkit.TextBlock{Text: "must-be-cleared"}},
			}},
		}
		mu.Lock()
		if system != "candidate prompt" {
			t.Errorf("NewConv system = %q, want candidate prompt", system)
		}
		conversations = append(conversations, conv)
		mu.Unlock()
		return conv, nil
	}
	cases := []folder.Case{
		{Name: "one", Dir: "/cases/one", Input: "first input"},
		{Name: "two", Dir: "/cases/two", Input: "second input"},
	}
	score := scorerFunc(func(_ context.Context, caseDir, output string) (scorer.Result, error) {
		return scorer.Result{Score: 1, Feedback: caseDir + ":" + output}, nil
	})

	got, err := Evaluate(context.Background(), nc, score, "candidate prompt", cases, 2)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if len(conversations) != 2 || conversations[0] == conversations[1] {
		t.Fatalf("conversations = %v, want two distinct instances", conversations)
	}
	sort.Strings(systems)
	if want := []string{"candidate prompt", "candidate prompt"}; !reflect.DeepEqual(systems, want) {
		t.Errorf("request systems = %q, want %q", systems, want)
	}
	sort.Strings(inputs)
	if want := []string{"first input", "second input"}; !reflect.DeepEqual(inputs, want) {
		t.Errorf("request inputs = %q, want %q", inputs, want)
	}
	if !reflect.DeepEqual(toolCounts, []int{0, 0}) {
		t.Errorf("request tool counts = %v, want bare calls with no tools", toolCounts)
	}
	if got.Cases[0].Output != "answer:first input" || got.Cases[1].Output != "answer:second input" {
		t.Errorf("outputs = [%q, %q], want final assistant texts", got.Cases[0].Output, got.Cases[1].Output)
	}
	if got.Cases[0].Feedback != "/cases/one:answer:first input" {
		t.Errorf("feedback = %q, want scorer feedback", got.Cases[0].Feedback)
	}
}

type concurrencyProbe struct {
	mu       sync.Mutex
	inFlight int
	max      int
	delays   map[string]time.Duration
}

func (p *concurrencyProbe) provider() providerFunc {
	return func(ctx context.Context, req *agentkit.Request) *agentkit.RoundTrip {
		input := requestText(req)
		p.mu.Lock()
		p.inFlight++
		if p.inFlight > p.max {
			p.max = p.inFlight
		}
		p.mu.Unlock()
		select {
		case <-ctx.Done():
		case <-time.After(p.delays[input]):
		}
		p.mu.Lock()
		p.inFlight--
		p.mu.Unlock()
		if err := ctx.Err(); err != nil {
			return failedRoundTrip(err)
		}
		return successfulRoundTrip("out-"+input, 1)
	}
}

func evaluateWithDelays(t *testing.T, delays map[string]time.Duration) (*EvalResult, int) {
	t.Helper()
	probe := &concurrencyProbe{delays: delays}
	nc := func(string) (*agentkit.Conversation, error) {
		return &agentkit.Conversation{Provider: probe.provider(), Model: "test-model"}, nil
	}
	score := scorerFunc(func(_ context.Context, _ string, output string) (scorer.Result, error) {
		return scorer.Result{Score: map[string]float64{"out-a": 1, "out-b": 2, "out-c": 3}[output]}, nil
	})
	cases := []folder.Case{{Name: "charlie", Input: "c"}, {Name: "alpha", Input: "a"}, {Name: "bravo", Input: "b"}}
	got, err := Evaluate(context.Background(), nc, score, "prompt", cases, 2)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	return got, probe.max
}

// R-QWJ1-KV6B
func TestEvaluateBoundsParallelCallsAndAggregatesDeterministically(t *testing.T) {
	first, firstMax := evaluateWithDelays(t, map[string]time.Duration{
		"a": 30 * time.Millisecond, "b": 5 * time.Millisecond, "c": 20 * time.Millisecond,
	})
	second, secondMax := evaluateWithDelays(t, map[string]time.Duration{
		"a": 5 * time.Millisecond, "b": 30 * time.Millisecond, "c": 10 * time.Millisecond,
	})
	if firstMax != 2 || secondMax != 2 {
		t.Fatalf("maximum calls in flight = %d and %d, want exactly pool limit 2", firstMax, secondMax)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("results differ by completion order:\nfirst  = %#v\nsecond = %#v", first, second)
	}
	if got := []string{first.Cases[0].Case.Name, first.Cases[1].Case.Name, first.Cases[2].Case.Name}; !reflect.DeepEqual(got, []string{"alpha", "bravo", "charlie"}) {
		t.Errorf("case order = %q, want sorted names", got)
	}
	if first.Composite != 2 {
		t.Errorf("composite = %v, want arithmetic mean 2", first.Composite)
	}
}

// R-QXQX-YMX0
func TestEvaluateModelFailureCancelsInflightAndPendingCases(t *testing.T) {
	slowStarted := make(chan struct{})
	slowCancelled := make(chan struct{})
	var onceStarted, onceCancelled sync.Once
	var mu sync.Mutex
	var called []string
	provider := providerFunc(func(ctx context.Context, req *agentkit.Request) *agentkit.RoundTrip {
		input := requestText(req)
		mu.Lock()
		called = append(called, input)
		mu.Unlock()
		switch input {
		case "slow":
			onceStarted.Do(func() { close(slowStarted) })
			<-ctx.Done()
			onceCancelled.Do(func() { close(slowCancelled) })
			return failedRoundTrip(ctx.Err())
		case "bad":
			select {
			case <-slowStarted:
			case <-time.After(time.Second):
				return failedRoundTrip(errors.New("slow case never started"))
			}
			return failedRoundTrip(errors.New("provider exploded"))
		default:
			return successfulRoundTrip("unexpected", 0)
		}
	})
	nc := func(string) (*agentkit.Conversation, error) {
		return &agentkit.Conversation{Provider: provider, Model: "test-model"}, nil
	}
	score := scorerFunc(func(context.Context, string, string) (scorer.Result, error) {
		return scorer.Result{}, errors.New("scorer must not be reached")
	})
	cases := []folder.Case{{Name: "failing-case", Input: "bad"}, {Name: "slow-case", Input: "slow"}, {Name: "pending-case", Input: "pending"}}

	got, err := Evaluate(context.Background(), nc, score, "prompt", cases, 2)
	if err == nil || got != nil {
		t.Fatalf("Evaluate() = (%#v, %v), want nil result and error", got, err)
	}
	if want := `case "failing-case": model call: provider exploded`; err.Error() != want {
		t.Fatalf("error = %q, want %q", err, want)
	}
	select {
	case <-slowCancelled:
	default:
		t.Fatal("in-flight slow call was not cancelled")
	}
	mu.Lock()
	defer mu.Unlock()
	sort.Strings(called)
	if !reflect.DeepEqual(called, []string{"bad", "slow"}) {
		t.Fatalf("called inputs = %q, want failed and in-flight cases only", called)
	}
}

// R-QYYU-CENP
func TestEvaluateSumsExactReportedCaseCosts(t *testing.T) {
	costs := map[string]agentkit.Cost{"one": 11, "two": 23, "three": 37}
	provider := providerFunc(func(_ context.Context, req *agentkit.Request) *agentkit.RoundTrip {
		input := requestText(req)
		return successfulRoundTrip(fmt.Sprintf("output-%s", input), costs[input])
	})
	nc := func(string) (*agentkit.Conversation, error) {
		return &agentkit.Conversation{Provider: provider, Model: "test-model"}, nil
	}
	score := scorerFunc(func(context.Context, string, string) (scorer.Result, error) {
		return scorer.Result{Score: 1}, nil
	})
	cases := []folder.Case{{Name: "one", Input: "one"}, {Name: "two", Input: "two"}, {Name: "three", Input: "three"}}

	got, err := Evaluate(context.Background(), nc, score, "prompt", cases, 3)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if got.Cost != 71 {
		t.Errorf("total cost = %d, want exact sum 71", got.Cost)
	}
	if got.Cases[0].Cost != 11 || got.Cases[1].Cost != 37 || got.Cases[2].Cost != 23 {
		t.Errorf("sorted per-case costs = [%d %d %d], want [11 37 23]", got.Cases[0].Cost, got.Cases[1].Cost, got.Cases[2].Cost)
	}
}
