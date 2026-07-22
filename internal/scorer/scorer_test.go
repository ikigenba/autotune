package scorer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// R-R06Q-Q6EE
func TestExecScorerPassesCaseDirectoryWorkingDirectoryAndModelOutput(t *testing.T) {
	workdir := t.TempDir()
	caseDir := filepath.Join(workdir, "cases", "first")
	if err := os.MkdirAll(caseDir, 0o755); err != nil {
		t.Fatal(err)
	}
	script := writeScript(t, workdir, `
if [ "$#" -ne 1 ]; then
  echo "wrong argument count" >&2
  exit 9
fi
model_output=$(cat)
printf '{"score":0.75,"feedback":"%s|%s|%s"}\n' "$PWD" "$1" "$model_output"
`)

	result, err := New(script, workdir).Score(context.Background(), caseDir, "model answer")
	if err != nil {
		t.Fatalf("Score() error = %v", err)
	}
	want := strings.Join([]string{workdir, caseDir, "model answer"}, "|")
	if result.Feedback != want {
		t.Fatalf("feedback = %q, want fixture echo %q", result.Feedback, want)
	}
}

// R-R1EN-3Y53
func TestExecScorerParsesScoreAndOptionalFeedback(t *testing.T) {
	tests := []struct {
		name       string
		output     string
		wantScore  float64
		wantDetail string
	}{
		{name: "with feedback", output: `{"score":0.625,"feedback":"use clearer names"}`, wantScore: 0.625, wantDetail: "use clearer names"},
		{name: "without feedback", output: `{"score":1}`, wantScore: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workdir := t.TempDir()
			script := writeScript(t, workdir, fmt.Sprintf("printf '%%s\\n' '%s'", tt.output))
			result, err := New(script, workdir).Score(context.Background(), workdir, "answer")
			if err != nil {
				t.Fatalf("Score() error = %v", err)
			}
			if result.Score != tt.wantScore || result.Feedback != tt.wantDetail {
				t.Fatalf("Score() = %+v, want score %v and feedback %q", result, tt.wantScore, tt.wantDetail)
			}
		})
	}
}

// R-R2MJ-HPVS
func TestExecScorerRejectsInvalidOutput(t *testing.T) {
	tests := []struct {
		name   string
		output string
	}{
		{name: "not JSON", output: `not-json`},
		{name: "missing score", output: `{"feedback":"none"}`},
		{name: "above range", output: `{"score":1.5}`},
		{name: "below range", output: `{"score":-0.1}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workdir := t.TempDir()
			script := writeScript(t, workdir, fmt.Sprintf("printf '%%s\\n' '%s'", tt.output))
			if _, err := New(script, workdir).Score(context.Background(), workdir, "answer"); err == nil {
				t.Fatal("Score() error = nil, want hard error")
			}
		})
	}
}

// R-R3UF-VHMH
func TestExecScorerReturnsStderrOnNonzeroExit(t *testing.T) {
	workdir := t.TempDir()
	script := writeScript(t, workdir, `
echo "scorer exploded" >&2
exit 7
`)

	_, err := New(script, workdir).Score(context.Background(), workdir, "answer")
	if err == nil {
		t.Fatal("Score() error = nil, want process error")
	}
	if !strings.Contains(err.Error(), "scorer exploded") {
		t.Fatalf("Score() error = %q, want scorer stderr", err)
	}
}

func writeScript(t *testing.T, dir, body string) string {
	t.Helper()
	path := filepath.Join(dir, "score")
	contents := "#!/bin/sh\nset -eu\n" + body + "\n"
	if err := os.WriteFile(path, []byte(contents), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}
