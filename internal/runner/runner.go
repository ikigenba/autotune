// Package runner evaluates candidate prompts against case sets.
package runner

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/ikigenba/agentkit"
	"github.com/ikigenba/autotune/internal/folder"
	"github.com/ikigenba/autotune/internal/scorer"
)

// CaseResult is the model output, score, feedback, and cost for one case.
type CaseResult struct {
	Case     folder.Case
	Output   string
	Score    float64
	Feedback string
	Cost     agentkit.Cost
}

// EvalResult is a deterministic scorecard for a complete case set.
type EvalResult struct {
	Cases     []CaseResult
	Composite float64
	Cost      agentkit.Cost
}

// NewConv builds a fresh conversation for one model call.
type NewConv func(system string) (*agentkit.Conversation, error)

// WarnFunc receives provider warnings after a stream has been drained.
type WarnFunc func(agentkit.Warning)

type outcome struct {
	result CaseResult
	err    error
}

// Evaluate runs every case, with at most parallel model calls in flight.
func Evaluate(ctx context.Context, nc NewConv, score scorer.Scorer, prompt string, cases []folder.Case, parallel int, warn WarnFunc) (*EvalResult, error) {
	if parallel < 1 {
		return nil, fmt.Errorf("parallel must be at least 1")
	}
	if nc == nil {
		return nil, fmt.Errorf("new conversation function is required")
	}
	if score == nil {
		return nil, fmt.Errorf("scorer is required")
	}
	if len(cases) == 0 {
		return &EvalResult{}, nil
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	workerCount := min(parallel, len(cases))
	jobs := make(chan folder.Case)
	outcomes := make(chan outcome, len(cases))
	var workers sync.WaitGroup
	workers.Add(workerCount)
	for range workerCount {
		go func() {
			defer workers.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case c, ok := <-jobs:
					if !ok {
						return
					}
					result, err := evaluateCase(ctx, nc, score, prompt, c, warn)
					outcomes <- outcome{result: result, err: err}
					if err != nil {
						return
					}
				}
			}
		}()
	}

	go func() {
		defer close(jobs)
		for _, c := range cases {
			select {
			case <-ctx.Done():
				return
			case jobs <- c:
			}
		}
	}()
	go func() {
		workers.Wait()
		close(outcomes)
	}()

	results := make([]CaseResult, 0, len(cases))
	var firstErr error
	for item := range outcomes {
		if item.err != nil {
			if firstErr == nil {
				firstErr = item.err
				cancel()
			}
			continue
		}
		results = append(results, item.result)
	}
	if firstErr != nil {
		return nil, firstErr
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(results) != len(cases) {
		return nil, fmt.Errorf("evaluation stopped after %d of %d cases", len(results), len(cases))
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Case.Name < results[j].Case.Name
	})
	final := &EvalResult{Cases: results}
	for _, result := range results {
		final.Composite += result.Score
		final.Cost += result.Cost
	}
	final.Composite /= float64(len(results))
	return final, nil
}

func evaluateCase(ctx context.Context, nc NewConv, score scorer.Scorer, prompt string, c folder.Case, warn WarnFunc) (CaseResult, error) {
	conv, err := nc(prompt)
	if err != nil {
		return CaseResult{}, fmt.Errorf("case %q: create conversation: %w", c.Name, err)
	}
	if conv == nil {
		return CaseResult{}, fmt.Errorf("case %q: create conversation: nil conversation", c.Name)
	}
	defer conv.Close()

	// Evaluation calls are deliberately bare and independent.
	conv.System = prompt
	conv.Tools = nil
	conv.DeferredTools = nil
	conv.MCPServers = nil
	conv.History = nil

	stream := conv.Send(ctx, c.Input)
	var output string
	foundDone := false
	for event := range stream.Events() {
		if done, ok := event.(agentkit.MessageDone); ok {
			output = messageText(done.Message)
			foundDone = true
		}
	}
	if warn != nil {
		for _, warning := range stream.Warnings() {
			warn(warning)
		}
	}
	if err := stream.Err(); err != nil {
		return CaseResult{}, fmt.Errorf("case %q: model call: %w", c.Name, err)
	}
	if !foundDone {
		return CaseResult{}, fmt.Errorf("case %q: model call produced no final assistant message", c.Name)
	}

	scored, err := score.Score(ctx, c.Dir, output)
	if err != nil {
		return CaseResult{}, fmt.Errorf("case %q: score: %w", c.Name, err)
	}
	return CaseResult{
		Case:     c,
		Output:   output,
		Score:    scored.Score,
		Feedback: scored.Feedback,
		Cost:     stream.Cost(),
	}, nil
}

func messageText(message agentkit.Message) string {
	var text strings.Builder
	for _, block := range message.Blocks {
		if block, ok := block.(agentkit.TextBlock); ok {
			text.WriteString(block.Text)
		}
	}
	return text.String()
}
