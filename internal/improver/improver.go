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

// ErrUnparseable marks a Propose failure in which the improver's replies could
// not be parsed after every attempt, as distinct from a hard call or context
// error. The tuning loop skips the iteration on ErrUnparseable and keeps going;
// any other Propose error still stops the run.
var ErrUnparseable = errors.New("improver response unparseable")

// Parse validates and extracts an improver response. It is lenient about the
// SUMMARY marker — leading whitespace and surrounding markdown emphasis
// (`**`, `*`, `_`, `__`, backticks) are tolerated, e.g. "**SUMMARY:** ..." —
// while keeping the contract unambiguous: exactly one SUMMARY line outside the
// fenced block, and exactly one fenced block whose contents are the complete
// replacement prompt. A SUMMARY-looking line inside the fence is part of the
// prompt, never the response summary.
func Parse(reply string) (summary, prompt string, err error) {
	lines := splitLines(reply)

	// Locate the fenced block first so the summary scan can ignore anything
	// inside the prompt.
	type fence struct {
		openLine, closeLine      int
		contentStart, contentEnd int
	}
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
				fences = append(fences, fence{openLine: i, closeLine: j, contentStart: start, contentEnd: lines[j].start})
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

	for idx, line := range lines {
		if idx >= f.openLine && idx <= f.closeLine {
			continue // inside the prompt fence
		}
		value, ok := summaryValue(line.text)
		if !ok {
			continue
		}
		if value == "" {
			return "", "", errors.New("improver response is missing a non-empty SUMMARY line")
		}
		if summary != "" {
			return "", "", errors.New("improver response contains multiple SUMMARY lines")
		}
		summary = value
	}
	if summary == "" {
		return "", "", errors.New("improver response is missing a non-empty SUMMARY line")
	}
	return summary, reply[f.contentStart:f.contentEnd], nil
}

// summaryValue reports whether a line carries the SUMMARY marker, tolerating
// leading whitespace and surrounding markdown emphasis, and returns the
// trimmed summary text (which may be empty for a bare marker).
func summaryValue(line string) (string, bool) {
	t := strings.TrimLeft(strings.TrimSpace(line), "*_`")
	rest, ok := strings.CutPrefix(t, "SUMMARY:")
	if !ok {
		return "", false
	}
	return strings.Trim(strings.TrimSpace(rest), "*_` "), true
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
func Propose(ctx context.Context, nc runner.NewConv, improveMD string, e Evidence, maxRetries int, warn runner.WarnFunc) (summary, prompt string, err error) {
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
		// Fresh conversation each attempt (no chat memory), but a retry carries
		// the prior parse error forward so the model can correct itself instead
		// of re-emitting the identical malformed reply.
		user := bundle
		if parseErr != nil {
			user = correction(parseErr) + bundle
		}
		reply, callErr := send(ctx, conversation, user, warn)
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
	return "", "", fmt.Errorf("improver response invalid after %d attempts (%w): %v", maxRetries+1, ErrUnparseable, parseErr)
}

// correction prefaces a retry bundle with the previous parse failure and a
// restatement of the required format.
func correction(parseErr error) string {
	return fmt.Sprintf("Your previous reply could not be used: %v.\n\n"+
		"Reply again in the exact required format: a single line beginning `SUMMARY:` "+
		"with a one-line description of the change, followed by exactly one fenced code "+
		"block containing the complete replacement prompt and nothing else. Do not add "+
		"extra fenced blocks and do not place the summary inside the fence.\n\n", parseErr)
}

func send(ctx context.Context, conversation *agentkit.Conversation, user string, warn runner.WarnFunc) (string, error) {
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
	if warn != nil {
		for _, warning := range stream.Warnings() {
			warn(warning)
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
