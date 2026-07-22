package config

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ikigenba/autotune/internal/cli"
)

func validFile(extra string) []byte {
	return []byte(`{"runner":{"provider":"openai","model":"runner-file"` + extra + `},"improver":{"provider":"anthropic","model":"improver-file"}}`)
}

// R-QMRU-IP8R
func TestResolveUsesLayerPrecedenceAndLaterCLIPairs(t *testing.T) {
	cfg, err := Resolve(validFile(""), []string{"runner.model=first", "runner.model=last"}, func(string) string { return "key" }, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Runner.Model != "last" || cfg.Improver.Model != "improver-file" {
		t.Fatalf("resolved models = runner %q, improver %q", cfg.Runner.Model, cfg.Improver.Model)
	}
}

// R-QNZQ-WGZG
func TestParsePairFirstEqualsAndUsageErrors(t *testing.T) {
	key, value, err := ParsePair("runner.base_url=https://example.test?a=b")
	if err != nil || key != "runner.base_url" || value != "https://example.test?a=b" {
		t.Fatalf("ParsePair = %q, %q, %v", key, value, err)
	}
	for _, raw := range []string{"missing", "=value", "key="} {
		_, _, err := ParsePair(raw)
		if err == nil || !cli.IsUsageError(err) || cli.Usage.ExitCode() != 2 {
			t.Errorf("ParsePair(%q) error = %v, want exit-2 usage error", raw, err)
		}
	}
}

// R-QQFJ-O0GU
func TestResolveRequiresCLISectionsAndKeepsThemIndependent(t *testing.T) {
	_, err := Resolve(validFile(""), []string{"model=x"}, func(string) string { return "key" }, t.TempDir())
	if err == nil || !cli.IsUsageError(err) || !strings.Contains(err.Error(), "runner.") || !strings.Contains(err.Error(), "improver.") {
		t.Fatalf("unprefixed key error = %v", err)
	}
	cfg, err := Resolve(validFile(""), []string{"runner.model=runner-cli", "improver.provider=google"}, func(string) string { return "key" }, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Runner.Model != "runner-cli" || cfg.Runner.Provider != "openai" || cfg.Improver.Model != "improver-file" || cfg.Improver.Provider != "google" {
		t.Fatalf("sections leaked values: %+v", cfg)
	}
}

// R-QRNG-1S7J
func TestResolveRejectsUnknownKeysInBothLayers(t *testing.T) {
	badFile := []byte(`{"runner":{"provider":"openai","model":"x","mystery":"y"},"improver":{"provider":"openai","model":"x"}}`)
	cases := []struct {
		name  string
		raw   []byte
		pairs []string
	}{
		{"file", badFile, nil},
		{"cli", validFile(""), []string{"runner.mystery=y"}},
	}
	for _, tc := range cases {
		_, err := Resolve(tc.raw, tc.pairs, func(string) string { return "key" }, t.TempDir())
		if err == nil || !strings.Contains(err.Error(), "mystery") {
			t.Errorf("%s unknown key error = %v", tc.name, err)
		}
	}
}

// R-QSVC-FJY8
func TestResolveMapsVocabularyAndRejectsMalformedValues(t *testing.T) {
	pairs := []string{
		"runner.temperature=0", "runner.top_p=0.75", "runner.max_tokens=123",
		"runner.base_delay=2s", "runner.thinking=false", "improver.effort=high",
		"runner.max_attempts=4", "runner.ignore_retry_after=true",
	}
	requestedEnv := ""
	cfg, err := Resolve(validFile(""), pairs, func(name string) string {
		requestedEnv = name
		return "secret"
	}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	conv, err := cfg.Runner.Conversation("system text")
	if err != nil {
		t.Fatal(err)
	}
	if conv.Gen.Temperature == nil || *conv.Gen.Temperature != 0 || conv.Gen.TopP == nil || *conv.Gen.TopP != .75 || conv.Gen.MaxTokens != 123 {
		t.Fatalf("generation settings = %+v", conv.Gen)
	}
	if conv.Retry.BaseDelay != 2*time.Second || conv.Retry.MaxAttempts != 4 || !conv.Retry.IgnoreRetryAfter || !conv.Gen.Reasoning.Disabled() || conv.System != "system text" {
		t.Fatalf("conversation mapping = Gen %+v Retry %+v System %q", conv.Gen, conv.Retry, conv.System)
	}
	if level, ok := cfg.Improver.Reasoning.Level(); !ok || level != "high" {
		t.Fatalf("improver reasoning level = %q, %v", level, ok)
	}
	if requestedEnv != "OPENAI_API_KEY" {
		t.Fatalf("credential environment lookup = %q", requestedEnv)
	}
	for _, pair := range []string{"runner.max_tokens=nope", "runner.base_delay=soon"} {
		_, err := Resolve(validFile(""), []string{pair}, func(string) string { return "secret" }, t.TempDir())
		if err == nil {
			t.Errorf("Resolve(%q) succeeded", pair)
		}
	}
}

// R-QU38-TBOX
func TestResolveValidatesAuthDefaultsFileAndRequiredFields(t *testing.T) {
	home := t.TempDir()
	cfg, err := Resolve(validFile(`,"auth":"sub"`), nil, func(string) string { return "" }, home)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Runner.AuthFile != filepath.Join(home, ".autotune", "auth.json") {
		t.Fatalf("AuthFile = %q", cfg.Runner.AuthFile)
	}
	for name, raw := range map[string][]byte{
		"auth":     validFile(`,"auth":"cookie"`),
		"provider": []byte(`{"runner":{"model":"x"},"improver":{"provider":"openai","model":"x"}}`),
		"model":    []byte(`{"runner":{"provider":"openai"},"improver":{"provider":"openai","model":"x"}}`),
	} {
		_, err := Resolve(raw, nil, func(string) string { return "" }, home)
		if err == nil || !strings.Contains(err.Error(), name) {
			t.Errorf("missing/invalid %s error = %v", name, err)
		}
	}
}
