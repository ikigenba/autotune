// Package scorer runs a tune folder's external scoring program.
package scorer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
)

// Result is the validated result returned by a scoring program.
type Result struct {
	Score    float64
	Feedback string
}

// Scorer scores model output for a case.
type Scorer interface {
	Score(ctx context.Context, caseDir, modelOutput string) (Result, error)
}

type execScorer struct {
	scorePath string
	workdir   string
}

// New returns a Scorer that invokes scorePath from workdir.
func New(scorePath, workdir string) Scorer {
	return &execScorer{scorePath: scorePath, workdir: workdir}
}

func (s *execScorer) Score(ctx context.Context, caseDir, modelOutput string) (Result, error) {
	absCaseDir, err := filepath.Abs(caseDir)
	if err != nil {
		return Result{}, fmt.Errorf("resolve case directory: %w", err)
	}

	cmd := exec.CommandContext(ctx, s.scorePath, absCaseDir)
	cmd.Dir = s.workdir
	cmd.Stdin = strings.NewReader(modelOutput)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if message := strings.TrimSpace(stderr.String()); message != "" {
			return Result{}, fmt.Errorf("run scorer: %w: %s", err, message)
		}
		return Result{}, fmt.Errorf("run scorer: %w", err)
	}

	var envelope struct {
		Score    *float64 `json:"score"`
		Feedback string   `json:"feedback"`
	}
	decoder := json.NewDecoder(&stdout)
	if err := decoder.Decode(&envelope); err != nil {
		return Result{}, fmt.Errorf("decode scorer output: %w", err)
	}
	if err := ensureJSONEnd(decoder); err != nil {
		return Result{}, fmt.Errorf("decode scorer output: %w", err)
	}
	if envelope.Score == nil {
		return Result{}, fmt.Errorf("decode scorer output: missing score")
	}
	if *envelope.Score < 0 || *envelope.Score > 1 {
		return Result{}, fmt.Errorf("scorer score %v is outside [0,1]", *envelope.Score)
	}

	return Result{Score: *envelope.Score, Feedback: envelope.Feedback}, nil
}

func ensureJSONEnd(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("unexpected data after JSON object")
		}
		return err
	}
	return nil
}
