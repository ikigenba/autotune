package improver

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ikigenba/agentkit"
	"github.com/ikigenba/autotune/internal/folder"
	"github.com/ikigenba/autotune/internal/runner"
)

// R-RETJ-BFAQ
func TestBuildBundleIncludesAllScoresOnlyWorstDetailsAndHistory(t *testing.T) {
	evidence := Evidence{
		Incumbent: "incumbent prompt",
		Baseline:  0.25, Best: 0.75, Epsilon: 0.01,
		Cases: []runner.CaseResult{
			{Case: folder.Case{Name: "zeta", Input: "ZETA INPUT"}, Output: "ZETA OUTPUT", Score: 0.8, Feedback: "ZETA FEEDBACK"},
			{Case: folder.Case{Name: "beta", Input: "BETA INPUT"}, Output: "BETA OUTPUT", Score: 0.2, Feedback: "BETA FEEDBACK"},
			{Case: folder.Case{Name: "alpha", Input: "ALPHA INPUT"}, Output: "ALPHA OUTPUT", Score: 0.2, Feedback: "ALPHA FEEDBACK"},
		},
		History: []Attempt{{Summary: "made it terse", Composite: 0.6}, {Summary: "added examples", Composite: 0.75, Accepted: true}},
		WorstK:  2,
	}
	bundle := BuildBundle(evidence)
	for _, want := range []string{"incumbent prompt", "Baseline: 0.25", "Current best: 0.75", "Epsilon: 0.01", "| zeta | 0.8 |", "| beta | 0.2 |", "| alpha | 0.2 |", "ALPHA INPUT", "ALPHA OUTPUT", "ALPHA FEEDBACK", "BETA INPUT", "BETA OUTPUT", "BETA FEEDBACK", "made it terse | composite 0.6 | rejected", "added examples | composite 0.75 | accepted"} {
		if !strings.Contains(bundle, want) {
			t.Errorf("bundle missing %q:\n%s", want, bundle)
		}
	}
	for _, excluded := range []string{"ZETA INPUT", "ZETA OUTPUT", "ZETA FEEDBACK", "HOLDOUT SECRET"} {
		if strings.Contains(bundle, excluded) {
			t.Errorf("bundle unexpectedly contains non-worst or holdout detail %q", excluded)
		}
	}
	if strings.Index(bundle, "## alpha") > strings.Index(bundle, "## beta") {
		t.Error("equal-scoring worst cases are not ordered by case name")
	}
}

// R-RG1F-P71F
func TestParseRequiresSummaryAndExactlyOneFenceAndPreservesContents(t *testing.T) {
	summary, prompt, err := Parse("notes\nSUMMARY: changed examples\n```text\n  leading\ntrailing  \n```\n")
	if err != nil {
		t.Fatal(err)
	}
	if summary != "changed examples" {
		t.Fatalf("summary = %q", summary)
	}
	if prompt != "  leading\ntrailing  \n" {
		t.Fatalf("prompt was not extracted verbatim: %q", prompt)
	}
	for name, reply := range map[string]string{
		"missing summary": "```\nprompt\n```",
		"missing fence":   "SUMMARY: change\nprompt",
		"two fences":      "SUMMARY: change\n```\na\n```\n```\nb\n```",
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, err := Parse(reply); err == nil {
				t.Fatal("Parse unexpectedly succeeded")
			}
		})
	}
}

// R-RH9C-2YS4
func TestProposeRetriesMalformedRepliesWithFreshCallsAndExhausts(t *testing.T) {
	t.Run("retry succeeds", func(t *testing.T) {
		provider := &scriptedProvider{replies: []string{"bad", "SUMMARY: fixed\n```\nnew prompt\n```"}}
		nc := newConversationFactory(provider)
		summary, prompt, err := Propose(context.Background(), nc, "improve system", Evidence{}, 1, nil)
		if err != nil {
			t.Fatal(err)
		}
		if summary != "fixed" || prompt != "new prompt\n" || provider.calls != 2 {
			t.Fatalf("got summary=%q prompt=%q calls=%d", summary, prompt, provider.calls)
		}
	})
	t.Run("exhaustion fails", func(t *testing.T) {
		provider := &scriptedProvider{replies: []string{"bad", "still bad", "also bad"}}
		_, _, err := Propose(context.Background(), newConversationFactory(provider), "system", Evidence{}, 2, nil)
		if err == nil || !strings.Contains(err.Error(), "invalid after 3 attempts") {
			t.Fatalf("expected exhaustion error, got %v", err)
		}
		if provider.calls != 3 {
			t.Fatalf("calls = %d, want 3", provider.calls)
		}
	})
}

// R-RIH8-GQIT
func TestProposeCallsAreSequentialFreshAndBare(t *testing.T) {
	provider := &scriptedProvider{replies: []string{"bad", "SUMMARY: valid\n```\nprompt\n```"}, delay: time.Millisecond}
	var conversations []*agentkit.Conversation
	nc := func(system string) (*agentkit.Conversation, error) {
		conversation := &agentkit.Conversation{Provider: provider, Model: "test", System: system}
		conversations = append(conversations, conversation)
		return conversation, nil
	}
	if _, _, err := Propose(context.Background(), nc, "instructions", Evidence{Incumbent: "old"}, 1, nil); err != nil {
		t.Fatal(err)
	}
	if len(conversations) != 2 || conversations[0] == conversations[1] {
		t.Fatalf("conversations were not fresh: %#v", conversations)
	}
	if provider.maxInFlight != 1 {
		t.Fatalf("max concurrent provider calls = %d", provider.maxInFlight)
	}
	for i, request := range provider.requests {
		if request.System != "instructions" || len(request.Tools) != 0 || len(request.Messages) != 1 {
			t.Errorf("request %d carried state or tools: system=%q tools=%d messages=%d", i, request.System, len(request.Tools), len(request.Messages))
		}
	}
}

type scriptedProvider struct {
	mu          sync.Mutex
	replies     []string
	requests    []*agentkit.Request
	calls       int
	inFlight    int
	maxInFlight int
	delay       time.Duration
}

func (p *scriptedProvider) Name() string { return "scripted" }

func (p *scriptedProvider) RoundTrip(_ context.Context, request *agentkit.Request) *agentkit.RoundTrip {
	p.mu.Lock()
	p.calls++
	p.inFlight++
	if p.inFlight > p.maxInFlight {
		p.maxInFlight = p.inFlight
	}
	p.requests = append(p.requests, request)
	index := p.calls - 1
	p.mu.Unlock()
	if p.delay > 0 {
		time.Sleep(p.delay)
	}
	p.mu.Lock()
	p.inFlight--
	p.mu.Unlock()
	if index >= len(p.replies) {
		return agentkit.NewRoundTrip(agentkit.Message{}, agentkit.FinishStop, agentkit.Usage{}, nil, errors.New("script exhausted"), 0, false)
	}
	message := agentkit.Message{Role: agentkit.RoleAssistant, Blocks: []agentkit.Block{agentkit.TextBlock{Text: p.replies[index]}}}
	return agentkit.NewRoundTrip(message, agentkit.FinishStop, agentkit.Usage{}, nil, nil, 0, false)
}

func newConversationFactory(provider agentkit.Provider) runner.NewConv {
	return func(system string) (*agentkit.Conversation, error) {
		return &agentkit.Conversation{Provider: provider, Model: "test", System: system}, nil
	}
}
