// Package config resolves runner and improver model configuration.
package config

import (
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ikigenba/agentkit"
	"github.com/ikigenba/agentkit/anthropic"
	"github.com/ikigenba/agentkit/catalog"
	"github.com/ikigenba/agentkit/google"
	"github.com/ikigenba/agentkit/openai"
	"github.com/ikigenba/agentkit/openai/subscription"
	"github.com/ikigenba/agentkit/openrouter"
	"github.com/ikigenba/agentkit/zai"
	"github.com/ikigenba/autotune/internal/cli"
)

type Section struct {
	Provider, Model, Auth, AuthFile string
	Temperature, TopP               *float64
	MaxTokens                       int
	Reasoning                       agentkit.ReasoningValue
	Retry                           agentkit.RetryPolicy
	BaseURL                         string

	getenv func(string) string
}

type Config struct {
	Runner   Section
	Improver Section
}

func ParsePair(raw string) (key, value string, err error) {
	key, value, ok := strings.Cut(raw, "=")
	if !ok || key == "" || value == "" {
		return "", "", &cli.UsageError{Message: fmt.Sprintf("invalid -c value %q: expected non-empty key=value", raw)}
	}
	return key, value, nil
}

func Resolve(configRaw []byte, cliPairs []string, getenv func(string) string, home string) (*Config, error) {
	if getenv == nil {
		return nil, fmt.Errorf("config: getenv is required")
	}
	cfg := &Config{
		Runner:   Section{getenv: getenv},
		Improver: Section{getenv: getenv},
	}
	if len(configRaw) != 0 {
		if err := applyFile(cfg, configRaw); err != nil {
			return nil, err
		}
	}
	for _, raw := range cliPairs {
		key, value, err := ParsePair(raw)
		if err != nil {
			return nil, err
		}
		name, field, ok := strings.Cut(key, ".")
		if !ok || (name != "runner" && name != "improver") {
			return nil, &cli.UsageError{Message: fmt.Sprintf("config key %q must use the runner. or improver. namespace", key)}
		}
		if field == "" {
			return nil, &cli.UsageError{Message: fmt.Sprintf("unknown config key %q", key)}
		}
		section := &cfg.Runner
		if name == "improver" {
			section = &cfg.Improver
		}
		if err := set(section, field, value); err != nil {
			return nil, &cli.UsageError{Message: fmt.Sprintf("%s.%v", name, err)}
		}
	}
	for _, item := range []struct {
		name    string
		section *Section
	}{
		{"runner", &cfg.Runner},
		{"improver", &cfg.Improver},
	} {
		name, section := item.name, item.section
		if section.Auth == "sub" && section.AuthFile == "" {
			section.AuthFile = filepath.Join(home, ".autotune", "auth.json")
		}
		if section.Provider == "" {
			return nil, fmt.Errorf("config: %s.provider is required", name)
		}
		if section.Model == "" {
			return nil, fmt.Errorf("config: %s.model is required", name)
		}
	}
	return cfg, nil
}

func applyFile(cfg *Config, raw []byte) error {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(raw, &root); err != nil {
		return fmt.Errorf("config.json: %w", err)
	}
	for name := range root {
		if name != "runner" && name != "improver" {
			return fmt.Errorf("config.json: unknown section %q", name)
		}
	}
	for _, item := range []struct {
		name    string
		section *Section
	}{
		{"runner", &cfg.Runner},
		{"improver", &cfg.Improver},
	} {
		sectionRaw, ok := root[item.name]
		if !ok {
			continue
		}
		var values map[string]string
		if err := json.Unmarshal(sectionRaw, &values); err != nil {
			return fmt.Errorf("config.json: %s must be an object of string values: %w", item.name, err)
		}
		for key, value := range values {
			if err := set(item.section, key, value); err != nil {
				return fmt.Errorf("config.json: %s.%v", item.name, err)
			}
		}
	}
	return nil
}

func set(section *Section, key, value string) error {
	switch key {
	case "provider":
		section.Provider = value
	case "model":
		section.Model = value
	case "auth":
		if value != "key" && value != "sub" {
			return fmt.Errorf("auth must be key or sub, got %q", value)
		}
		section.Auth = value
	case "auth_file":
		section.AuthFile = value
	case "temperature":
		n, err := finiteFloat(key, value)
		if err != nil {
			return err
		}
		section.Temperature = &n
	case "top_p":
		n, err := finiteFloat(key, value)
		if err != nil {
			return err
		}
		section.TopP = &n
	case "max_tokens":
		n, err := nonnegativeInt(key, value)
		if err != nil {
			return err
		}
		section.MaxTokens = n
	case "effort":
		section.Reasoning = agentkit.Level(value)
	case "thinking_budget":
		n, err := nonnegativeInt(key, value)
		if err != nil {
			return err
		}
		section.Reasoning = agentkit.Budget(n)
	case "thinking":
		enabled, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("%s=%q is not a boolean", key, value)
		}
		if !enabled {
			section.Reasoning = agentkit.DisableReasoning()
		} else {
			section.Reasoning = agentkit.ReasoningValue{}
		}
	case "base_url":
		section.BaseURL = value
	case "max_attempts":
		n, err := nonnegativeInt(key, value)
		if err != nil {
			return err
		}
		section.Retry.MaxAttempts = n
	case "base_delay", "max_delay", "max_elapsed":
		d, err := time.ParseDuration(value)
		if err != nil || d < 0 {
			return fmt.Errorf("%s=%q is not a non-negative duration", key, value)
		}
		switch key {
		case "base_delay":
			section.Retry.BaseDelay = d
		case "max_delay":
			section.Retry.MaxDelay = d
		case "max_elapsed":
			section.Retry.MaxElapsed = d
		}
	case "ignore_retry_after":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("%s=%q is not a boolean", key, value)
		}
		section.Retry.IgnoreRetryAfter = b
	default:
		return fmt.Errorf("unknown config key %q", key)
	}
	return nil
}

func finiteFloat(key, value string) (float64, error) {
	n, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
		return 0, fmt.Errorf("%s=%q is not a finite number", key, value)
	}
	return n, nil
}

func nonnegativeInt(key, value string) (int, error) {
	n, err := strconv.Atoi(value)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("%s=%q is not a non-negative integer", key, value)
	}
	return n, nil
}

func (s *Section) Conversation(system string) (*agentkit.Conversation, error) {
	if s == nil {
		return nil, fmt.Errorf("config: nil section")
	}
	if s.getenv == nil {
		return nil, fmt.Errorf("config: section was not created by Resolve")
	}
	provider, err := s.provider()
	if err != nil {
		return nil, err
	}
	var pricing *agentkit.Pricing
	if entry, ok := catalog.Lookup(s.Model); ok {
		pricing = entry.Pricing
	}
	return &agentkit.Conversation{
		Provider: provider,
		Model:    s.Model,
		Pricing:  pricing,
		System:   system,
		Gen: agentkit.GenSettings{
			Temperature: s.Temperature,
			TopP:        s.TopP,
			MaxTokens:   s.MaxTokens,
			Reasoning:   s.Reasoning,
		},
		Retry: s.Retry,
	}, nil
}

// PricingPrecheck verifies that a positive spend rail can account for every
// configured conversation before any model call is made.
func (c *Config) PricingPrecheck(maxSpend float64) error {
	if maxSpend == 0 {
		return nil
	}
	if c == nil {
		return fmt.Errorf("config: nil config")
	}
	for _, item := range []struct {
		name  string
		model string
	}{
		{name: "runner", model: c.Runner.Model},
		{name: "improver", model: c.Improver.Model},
	} {
		entry, ok := catalog.Lookup(item.model)
		if !ok || entry.Pricing == nil {
			return fmt.Errorf("config: cannot arm spend rail: %s model %q has no catalog pricing", item.name, item.model)
		}
	}
	return nil
}

func (s *Section) provider() (agentkit.Provider, error) {
	if s.Auth == "sub" {
		if s.Provider != "openai" {
			return nil, fmt.Errorf("config: subscription auth is only supported by provider openai")
		}
		store, err := subscription.Load(s.AuthFile)
		if err != nil {
			return nil, fmt.Errorf("config: load subscription auth %q: %w", s.AuthFile, err)
		}
		opts := []openai.Option{}
		if s.BaseURL != "" {
			opts = append(opts, openai.WithBaseURL(s.BaseURL))
		}
		return openai.New(openai.Subscription(store), opts...), nil
	}

	envName := map[string]string{
		"openai":     "OPENAI_API_KEY",
		"anthropic":  "ANTHROPIC_API_KEY",
		"google":     "GOOGLE_API_KEY",
		"openrouter": "OPENROUTER_API_KEY",
		"zai":        "ZAI_API_KEY",
	}[s.Provider]
	if envName == "" {
		return nil, fmt.Errorf("config: unknown provider %q", s.Provider)
	}
	key := s.getenv(envName)
	if key == "" {
		return nil, fmt.Errorf("config: %s is required for provider %s", envName, s.Provider)
	}
	switch s.Provider {
	case "openai":
		opts := []openai.Option{}
		if s.BaseURL != "" {
			opts = append(opts, openai.WithBaseURL(s.BaseURL))
		}
		return openai.New(openai.APIKey(key), opts...), nil
	case "anthropic":
		opts := []anthropic.Option{}
		if s.BaseURL != "" {
			opts = append(opts, anthropic.WithBaseURL(s.BaseURL))
		}
		return anthropic.New(anthropic.APIKey(key), opts...), nil
	case "google":
		opts := []google.Option{}
		if s.BaseURL != "" {
			opts = append(opts, google.WithBaseURL(s.BaseURL))
		}
		return google.New(google.APIKey(key), opts...), nil
	case "openrouter":
		opts := []openrouter.Option{}
		if s.BaseURL != "" {
			opts = append(opts, openrouter.WithBaseURL(s.BaseURL))
		}
		return openrouter.New(openrouter.APIKey(key), opts...), nil
	case "zai":
		opts := []zai.Option{}
		if s.BaseURL != "" {
			opts = append(opts, zai.WithBaseURL(s.BaseURL))
		}
		return zai.New(zai.APIKey(key), opts...), nil
	default:
		panic("config: provider validation drift")
	}
}
