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
	bin := buildBinary(t)

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

// R-9A0E-BG7N
func TestCompiledBinaryDefaultsToDevVersion(t *testing.T) {
	bin := buildBinary(t)
	for _, flag := range []string{"-V", "--version"} {
		cmd := exec.Command(bin, flag)
		stdout, err := cmd.Output()
		if err != nil {
			t.Fatalf("%s: %v", flag, err)
		}
		if got, want := string(stdout), "dev\n"; got != want {
			t.Errorf("%s stdout = %q, want %q", flag, got, want)
		}
	}
}

// R-9B8A-P7YC
func TestCompiledBinaryAcceptsLinkerStampedVersion(t *testing.T) {
	const sentinel = "v9.8.7-test"
	bin := buildBinary(t, "-ldflags", "-X main.version="+sentinel)
	cmd := exec.Command(bin, "-V")
	stdout, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(stdout), sentinel+"\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func buildBinary(t *testing.T, args ...string) string {
	t.Helper()
	name := "autotune"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	bin := filepath.Join(t.TempDir(), name)
	buildArgs := append([]string{"build", "-o", bin}, args...)
	buildArgs = append(buildArgs, ".")
	build := exec.Command("go", buildArgs...)
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, output)
	}
	return bin
}
