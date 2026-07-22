package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// R-RYBX-FR5U
func TestCompiledBinaryCLIEnvelope(t *testing.T) {
	name := "autotune"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	bin := filepath.Join(t.TempDir(), name)
	build := exec.Command("go", "build", "-o", bin, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, output)
	}

	run := func(args ...string) (int, string, string) {
		t.Helper()
		var stdout, stderr bytes.Buffer
		cmd := exec.Command(bin, args...)
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		err := cmd.Run()
		if err == nil {
			return 0, stdout.String(), stderr.String()
		}
		if exit, ok := err.(*exec.ExitError); ok {
			return exit.ExitCode(), stdout.String(), stderr.String()
		}
		t.Fatalf("run %v: %v", args, err)
		return -1, "", ""
	}

	if code, stdout, stderr := run("--help"); code != 0 || !strings.Contains(stdout, "autotune --init") || stderr != "" {
		t.Fatalf("help: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if code, stdout, stderr := run("--version"); code != 0 || !strings.Contains(stdout, "autotune") || stderr != "" {
		t.Fatalf("version: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	root := filepath.Join(t.TempDir(), "tune")
	if code, _, stderr := run("--init", root); code != 0 || stderr != "" {
		t.Fatalf("init: code=%d stderr=%q", code, stderr)
	}
	for _, name := range []string{"prompt.txt", "improve.md", "config.json", "score", "cases/dev/example/input.txt", "cases/holdout"} {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Errorf("init missing %s: %v", name, err)
		}
	}
	if code, stdout, stderr := run("--unknown"); code != 2 || stdout != "" || !strings.Contains(stderr, "unknown flag") {
		t.Fatalf("usage error: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}
