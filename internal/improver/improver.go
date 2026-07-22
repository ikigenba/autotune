// Package improver builds evidence for, and requests, replacement prompts.
package improver

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/ikigenba/agentkit"
	"github.com/ikigenba/autotune/internal/runner"
)

// Attempt records the result of a previously proposed prompt.
type Attempt struct {
	Summary   string
	Composite float64
	Accepted  bool
}

// Evidence is the complete, development-only input to one improvement call.
type Evidence struct {
	Incumbent               string
	Baseline, Best, Epsilon float64
	Cases                   []runner.CaseResult
	History                 []Attempt
	WorstK                  int
}

// BuildBundle renders the bounded evidence supplied to the improver.
func BuildBundle(e Evidence) string {
	var b strings.Builder
	b.WriteString("# Incumbent prompt\n\n```\n")
	b.WriteString(e.Incumbent)
	b.WriteString("\n```\n\n# Score summary\n\n")
	fmt.Fprintf(&b, "Baseline: %s\nCurrent best: %s\nEpsilon: %s\n\n", number(e.Baseline), number(e.Best), number(e.Epsilon))
	b.WriteString("| Case | Score |\n| --- | ---: |\n")
	for _, result := range e.Cases {
		fmt.Fprintf(&b, "| %s | %s |\n", tableCell(result.Case.Name), number(result.Score))
	}

	worst := append([]runner.CaseResult(nil), e.Cases...)
	sort.SliceStable(worst, func(i, j int) bool {
		if worst[i].Score == worst[j].Score {
			return worst[i].Case.Name < worst[j].Case.Name
		}
		return worst[i].Score < worst[j].Score
	})
	k := e.WorstK
	if k < 0 {
		k = 0
	}
	if k > len(worst) {
		k = len(worst)
	}
	b.WriteString("\n# Worst cases\n")
	for _, result := range worst[:k] {
		fmt.Fprintf(&b, "\n## %s\n\nInput:\n```\n%s\n```\n\nModel output:\n```\n%s\n```\n\nScore: %s\n\nScorer feedback:\n```\n%s\n```\n", result.Case.Name, result.Case.Input, result.Output, number(result.Score), result.Feedback)
	}

	b.WriteString("\n# Attempt history\n")
	for _, attempt := range e.History {
		verdict := "rejected"
		if attempt.Accepted {
			verdict = "accepted"
		}
		fmt.Fprintf(&b, "\n- %s | composite %s | %s", attempt.Summary, number(attempt.Composite), verdict)
	}
	b.WriteByte('\n')
	return b.String()
}

func number(value float64) string {
	return strconv.FormatFloat(value, 'g', -1, 64)
}

func tableCell(value string) string {
	return strings.ReplaceAll(value, "|", "\\|")
}

// Parse validates and extracts an improver response.
func Parse(reply string) (summary, prompt string, err error) {
	lines := splitLines(reply)
	for _, line := range lines {
		if strings.HasPrefix(line.text, "SUMMARY:") {
			if summary != "" {
				return "", "", errors.New("improver response contains multiple SUMMARY lines")
			}
			summary = strings.TrimSpace(strings.TrimPrefix(line.text, "SUMMARY:"))
		}
	}
	if summary == "" {
		return "", "", errors.New("improver response is missing a non-empty SUMMARY line")
	}

	type fence struct{ contentStart, contentEnd int }
	var fences []fence
	for i := 0; i < len(lines); i++ {
		if !strings.HasPrefix(lines[i].text, "```") {
			continue
		}
		start := lines[i].end
		if start < len(reply) && reply[start] == '\r' {
			start++
		}
		if start < len(reply) && reply[start] == '\n' {
			start++
		}
		found := false
		for j := i + 1; j < len(lines); j++ {
			if strings.TrimSpace(lines[j].text) == "```" {
				fences = append(fences, fence{contentStart: start, contentEnd: lines[j].start})
				i = j
				found = true
				break
			}
		}
		if !found {
			return "", "", errors.New("improver response contains an unclosed fence")
		}
	}
	if len(fences) != 1 {
		return "", "", fmt.Errorf("improver response must contain exactly one fenced block, got %d", len(fences))
	}
	f := fences[0]
	return summary, reply[f.contentStart:f.contentEnd], nil
}

type lineSpan struct {
	text       string
	start, end int
}

func splitLines(s string) []lineSpan {
	var lines []lineSpan
	for start := 0; start <= len(s); {
		end := strings.IndexByte(s[start:], '\n')
		if end < 0 {
			end = len(s)
			lines = append(lines, lineSpan{text: strings.TrimSuffix(s[start:end], "\r"), start: start, end: end})
			break
		}
		end += start
		lines = append(lines, lineSpan{text: strings.TrimSuffix(s[start:end], "\r"), start: start, end: end})
		start = end + 1
	}
	return lines
}

// Propose makes a fresh, tool-free call for each parse attempt.
func Propose(ctx context.Context, nc runner.NewConv, improveMD string, e Evidence, maxRetries int) (summary, prompt string, err error) {
	if nc == nil {
		return "", "", errors.New("improver: nil conversation factory")
	}
	if maxRetries < 0 {
		return "", "", errors.New("improver: max retries must not be negative")
	}
	bundle := BuildBundle(e)
	var parseErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		conversation, convErr := nc(improveMD)
		if convErr != nil {
			return "", "", fmt.Errorf("improver: create conversation: %w", convErr)
		}
		if conversation == nil {
			return "", "", errors.New("improver: conversation factory returned nil")
		}
		if len(conversation.Tools) != 0 || len(conversation.DeferredTools) != 0 {
			_ = conversation.Close()
			return "", "", errors.New("improver: conversation must not have tools")
		}
		reply, callErr := send(ctx, conversation, bundle)
		closeErr := conversation.Close()
		if callErr != nil {
			return "", "", fmt.Errorf("improver call: %w", callErr)
		}
		if closeErr != nil {
			return "", "", fmt.Errorf("improver close: %w", closeErr)
		}
		summary, prompt, parseErr = Parse(reply)
		if parseErr == nil {
			return summary, prompt, nil
		}
	}
	return "", "", fmt.Errorf("improver response invalid after %d attempts: %w", maxRetries+1, parseErr)
}

func send(ctx context.Context, conversation *agentkit.Conversation, user string) (string, error) {
	stream := conversation.Send(ctx, user)
	var reply string
	for event := range stream.Events() {
		switch done := event.(type) {
		case agentkit.MessageDone:
			reply = messageText(done.Message)
		case *agentkit.MessageDone:
			reply = messageText(done.Message)
		}
	}
	if err := stream.Err(); err != nil {
		return "", err
	}
	return reply, nil
}

func messageText(message agentkit.Message) string {
	var b strings.Builder
	for _, block := range message.Blocks {
		if text, ok := block.(agentkit.TextBlock); ok {
			b.WriteString(text.Text)
		}
	}
	return b.String()
}
