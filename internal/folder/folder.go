package folder

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Case is a scorer case discovered in a tune folder.
type Case struct {
	Name  string
	Dir   string
	Input string
}

// Folder is a loaded and validated tune folder.
type Folder struct {
	Root      string
	Prompt    string
	ImproveMD string
	ScorePath string
	ConfigRaw []byte
	Dev       []Case
	Holdout   []Case
}

// Load reads and validates the tune folder rooted at root.
func Load(root string) (*Folder, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve tune folder: %w", err)
	}

	prompt, err := readRequiredFile(absRoot, "prompt.txt")
	if err != nil {
		return nil, err
	}
	improveMD, err := readRequiredFile(absRoot, "improve.md")
	if err != nil {
		return nil, err
	}
	configRaw, err := readRequiredFile(absRoot, "config.json")
	if err != nil {
		return nil, err
	}

	scorePath := filepath.Join(absRoot, "score")
	scoreInfo, err := os.Stat(scorePath)
	if err != nil {
		return nil, namedPathError("score", err)
	}
	if !scoreInfo.Mode().IsRegular() {
		return nil, fmt.Errorf("score is not a regular file")
	}
	if scoreInfo.Mode().Perm()&0o111 == 0 {
		return nil, fmt.Errorf("score is not executable")
	}

	dev, err := loadCases(filepath.Join(absRoot, "cases", "dev"))
	if err != nil {
		return nil, fmt.Errorf("dev cases: %w", err)
	}
	if len(dev) == 0 {
		return nil, fmt.Errorf("at least one dev case with input.txt is required")
	}
	holdout, err := loadCases(filepath.Join(absRoot, "cases", "holdout"))
	if err != nil {
		return nil, fmt.Errorf("holdout cases: %w", err)
	}

	return &Folder{
		Root:      absRoot,
		Prompt:    string(prompt),
		ImproveMD: string(improveMD),
		ScorePath: scorePath,
		ConfigRaw: configRaw,
		Dev:       dev,
		Holdout:   holdout,
	}, nil
}

func readRequiredFile(root, name string) ([]byte, error) {
	contents, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		return nil, namedPathError(name, err)
	}
	return contents, nil
}

func namedPathError(name string, err error) error {
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("missing %s: %w", name, err)
	}
	return fmt.Errorf("read %s: %w", name, err)
}

func loadCases(root string) ([]Case, error) {
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	cases := make([]Case, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(root, entry.Name())
		input, err := os.ReadFile(filepath.Join(dir, "input.txt"))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read %s input.txt: %w", entry.Name(), err)
		}
		cases = append(cases, Case{Name: entry.Name(), Dir: dir, Input: string(input)})
	}
	sort.Slice(cases, func(i, j int) bool { return cases[i].Name < cases[j].Name })
	return cases, nil
}

const configTemplate = `{
  "runner":   {"provider": "openai", "model": "gpt-5.6-luna", "auth": "sub", "temperature": "0"},
  "improver": {"provider": "openai", "model": "gpt-5.6-sol",  "auth": "sub", "effort": "high"}
}
`

const scoreTemplate = `#!/usr/bin/env bash
# Replace this placeholder with a scorer that reads the documented inputs.
echo 0
`

var scaffoldFiles = []struct {
	path string
	data string
	mode os.FileMode
}{
	{path: "prompt.txt", data: "", mode: 0o644},
	{path: "improve.md", data: "", mode: 0o644},
	{path: "score", data: scoreTemplate, mode: 0o755},
	{path: "config.json", data: configTemplate, mode: 0o644},
	{path: filepath.Join("cases", "dev", "example", "input.txt"), data: "", mode: 0o644},
	{path: filepath.Join("cases", "holdout", ".gitkeep"), data: "", mode: 0o644},
	{path: ".gitignore", data: "runs/\n", mode: 0o644},
}

// Init scaffolds a tune folder. It refuses to modify a directory containing
// any entry.
func Init(root string) error {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return fmt.Errorf("create tune folder: %w", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return fmt.Errorf("inspect tune folder: %w", err)
	}
	if len(entries) != 0 {
		return fmt.Errorf("tune folder is not empty")
	}

	for _, file := range scaffoldFiles {
		path := filepath.Join(root, file.path)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("create scaffold directory for %s: %w", file.path, err)
		}
		if err := os.WriteFile(path, []byte(file.data), file.mode); err != nil {
			return fmt.Errorf("write scaffold file %s: %w", file.path, err)
		}
		if err := os.Chmod(path, file.mode); err != nil {
			return fmt.Errorf("set scaffold mode for %s: %w", file.path, err)
		}
	}
	return nil
}
