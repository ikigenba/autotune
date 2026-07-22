package folder

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ikigenba/autotune/internal/config"
)

func TestInitCreatesExactSkeleton(t *testing.T) {
	// R-QFGG-82SL
	root := filepath.Join(t.TempDir(), "new-tune")
	if err := Init(root); err != nil {
		t.Fatalf("Init: %v", err)
	}

	var got []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			rel += "/"
		}
		got = append(got, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatalf("walk scaffold: %v", err)
	}
	want := []string{
		".gitignore",
		"cases/",
		"cases/dev/",
		"cases/dev/example/",
		"cases/dev/example/input.txt",
		"cases/holdout/",
		"cases/holdout/.gitkeep",
		"config.json",
		"improve.md",
		"prompt.txt",
		"score",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("scaffold entries:\n got: %q\nwant: %q", got, want)
	}
}

func TestInitRefusesNonEmptyDirectoryWithoutWriting(t *testing.T) {
	// R-QGOC-LUJA
	root := t.TempDir()
	existing := filepath.Join(root, "keep-me")
	if err := os.WriteFile(existing, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := Init(root)
	if err == nil || !strings.Contains(err.Error(), "not empty") {
		t.Fatalf("Init error = %v, want not-empty error", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "keep-me" {
		t.Fatalf("directory changed after refusal: %v", entries)
	}
	contents, err := os.ReadFile(existing)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "original" {
		t.Fatalf("existing file changed to %q", contents)
	}
}

func TestInitConfigResolvesToPinnedDefaults(t *testing.T) {
	// R-QHW8-ZM9Z
	root := t.TempDir()
	if err := Init(root); err != nil {
		t.Fatalf("Init: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := config.Resolve(raw, nil, func(string) string { return "" }, t.TempDir())
	if err != nil {
		t.Fatalf("Resolve scaffold config: %v", err)
	}
	if got.Runner.Provider != "openai" || got.Runner.Model != "gpt-5.6-luna" || got.Runner.Auth != "sub" {
		t.Fatalf("runner identity = %#v", got.Runner)
	}
	if got.Runner.Temperature == nil || *got.Runner.Temperature != 0 {
		t.Fatalf("runner temperature = %v, want 0", got.Runner.Temperature)
	}
	if got.Improver.Provider != "openai" || got.Improver.Model != "gpt-5.6-sol" || got.Improver.Auth != "sub" {
		t.Fatalf("improver identity = %#v", got.Improver)
	}
	if !strings.Contains(fmt.Sprint(got.Improver.Reasoning), "high") {
		t.Fatalf("improver effort = %v, want high", got.Improver.Reasoning)
	}
}

func TestInitWritesGitignoreAndExecutableScore(t *testing.T) {
	// R-QJ45-DE0O
	root := t.TempDir()
	if err := Init(root); err != nil {
		t.Fatalf("Init: %v", err)
	}
	ignore, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if string(ignore) != "runs/\n" {
		t.Fatalf(".gitignore = %q, want runs/", ignore)
	}
	score, err := os.Stat(filepath.Join(root, "score"))
	if err != nil {
		t.Fatal(err)
	}
	if score.Mode().Perm()&0o111 == 0 {
		t.Fatalf("score mode = %v, want executable", score.Mode())
	}
}

func TestLoadNamesEachMissingOrInvalidRequirement(t *testing.T) {
	// R-QKC1-R5RD
	tests := []struct {
		name          string
		breakScaffold func(t *testing.T, root string)
		want          string
	}{
		{name: "prompt", breakScaffold: remove("prompt.txt"), want: "prompt.txt"},
		{name: "improver", breakScaffold: remove("improve.md"), want: "improve.md"},
		{name: "config", breakScaffold: remove("config.json"), want: "config.json"},
		{name: "score", breakScaffold: remove("score"), want: "score"},
		{name: "score executable", breakScaffold: func(t *testing.T, root string) {
			if err := os.Chmod(filepath.Join(root, "score"), 0o644); err != nil {
				t.Fatal(err)
			}
		}, want: "executable"},
		{name: "dev case", breakScaffold: remove(filepath.Join("cases", "dev", "example", "input.txt")), want: "dev case"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			if err := Init(root); err != nil {
				t.Fatalf("Init: %v", err)
			}
			tt.breakScaffold(t, root)
			_, err := Load(root)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Load error = %v, want error naming %q", err, tt.want)
			}
		})
	}

	root := t.TempDir()
	if err := Init(root); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if _, err := Load(root); err != nil {
		t.Fatalf("Load with empty holdout: %v", err)
	}
}

func TestLoadSortsCasesAndIgnoresNonDirectories(t *testing.T) {
	// R-QLJY-4XI2
	root := t.TempDir()
	if err := Init(root); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := os.RemoveAll(filepath.Join(root, "cases", "dev", "example")); err != nil {
		t.Fatal(err)
	}
	writeCase(t, root, "dev", "zebra", "dev-z")
	writeCase(t, root, "dev", "alpha", "dev-a")
	writeCase(t, root, "holdout", "later", "holdout-l")
	writeCase(t, root, "holdout", "before", "holdout-b")
	if err := os.WriteFile(filepath.Join(root, "cases", "dev", "README"), []byte("ignore"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	assertCases(t, got.Dev, []string{"alpha", "zebra"}, []string{"dev-a", "dev-z"})
	assertCases(t, got.Holdout, []string{"before", "later"}, []string{"holdout-b", "holdout-l"})
	if !filepath.IsAbs(got.ScorePath) || !filepath.IsAbs(got.Root) {
		t.Fatalf("paths are not absolute: root=%q score=%q", got.Root, got.ScorePath)
	}
	if got.Prompt != "" || got.ImproveMD != "" || len(got.ConfigRaw) == 0 {
		t.Fatalf("folder contents not populated: %#v", got)
	}
}

func remove(path string) func(*testing.T, string) {
	return func(t *testing.T, root string) {
		t.Helper()
		if err := os.Remove(filepath.Join(root, path)); err != nil {
			t.Fatal(err)
		}
	}
}

func writeCase(t *testing.T, root, split, name, input string) {
	t.Helper()
	dir := filepath.Join(root, "cases", split, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "input.txt"), []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertCases(t *testing.T, got []Case, names, inputs []string) {
	t.Helper()
	if len(got) != len(names) {
		t.Fatalf("case count = %d, want %d: %#v", len(got), len(names), got)
	}
	for i := range got {
		if got[i].Name != names[i] || got[i].Input != inputs[i] {
			t.Fatalf("case %d = %#v, want name=%q input=%q", i, got[i], names[i], inputs[i])
		}
		if filepath.Base(got[i].Dir) != names[i] {
			t.Fatalf("case %d dir = %q, want basename %q", i, got[i].Dir, names[i])
		}
	}
}
