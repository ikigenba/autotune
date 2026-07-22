// Package cli owns autotune's command-line surface.
package cli

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

const UsageText = `Usage:
  autotune --init <folder>
  autotune [flags] <folder>

Flags:
  -c section.key=value   repeatable config override
  --repeat N             baseline repeats (default 3)
  --parallel N           concurrent evaluations (default 4)
  --worst N              worst cases in bundle (default 5)
  --max-retries N        malformed reply retries (default 3)
  --max-iterations N     0 = unlimited (default 0)
  --max-time D           Go duration; 0 = unlimited
  --max-spend USD        0 = unlimited
  -h, --help             show help
  -V, --version          show version
`

type Options struct {
	Init          bool
	Folder        string
	Config        []string
	Repeat        int
	Parallel      int
	Worst         int
	MaxRetries    int
	MaxIterations int
	MaxTime       time.Duration
	MaxSpend      float64
	Help          bool
	Version       bool
}

type StopReason int

const (
	Done StopReason = iota
	Failed
	Usage
	RailCrossed
	Interrupted
)

func (r StopReason) ExitCode() int {
	switch r {
	case Done:
		return 0
	case Failed:
		return 1
	case Usage:
		return 2
	case RailCrossed:
		return 3
	case Interrupted:
		return 130
	default:
		panic(fmt.Sprintf("cli: invalid stop reason %d", r))
	}
}

type UsageError struct {
	Message string
}

func (e *UsageError) Error() string { return e.Message }

func IsUsageError(err error) bool {
	var target *UsageError
	return errors.As(err, &target)
}

func Parse(args []string) (Options, error) {
	opts := Options{Repeat: 3, Parallel: 4, Worst: 5, MaxRetries: 3}
	positionals := make([]string, 0, 1)

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-h", "--help":
			opts.Help = true
			continue
		case "-V", "--version":
			opts.Version = true
			continue
		case "--init":
			opts.Init = true
			continue
		case "--":
			positionals = append(positionals, args[i+1:]...)
			i = len(args)
			continue
		}

		name, inline, hasInline := strings.Cut(arg, "=")
		if name == "-c" || isValueFlag(name) {
			var value string
			if hasInline {
				value = inline
			} else {
				i++
				if i >= len(args) {
					return Options{}, usageError("%s requires a value", name)
				}
				value = args[i]
			}
			if value == "" {
				return Options{}, usageError("%s requires a value", name)
			}
			if err := setValue(&opts, name, value); err != nil {
				return Options{}, err
			}
			continue
		}

		if strings.HasPrefix(arg, "-") {
			return Options{}, usageError("unknown flag %q", arg)
		}
		positionals = append(positionals, arg)
	}

	if opts.Help && opts.Version {
		return Options{}, usageError("--help and --version cannot be used together")
	}
	if len(positionals) > 1 {
		return Options{}, usageError("expected one folder, got %d", len(positionals))
	}
	if len(positionals) == 1 {
		opts.Folder = positionals[0]
	} else if !opts.Help && !opts.Version {
		return Options{}, usageError("missing folder")
	}
	return opts, nil
}

func isValueFlag(name string) bool {
	switch name {
	case "--repeat", "--parallel", "--worst", "--max-retries", "--max-iterations", "--max-time", "--max-spend":
		return true
	default:
		return false
	}
}

func setValue(opts *Options, name, value string) error {
	if name == "-c" {
		opts.Config = append(opts.Config, value)
		return nil
	}
	if name == "--max-time" {
		duration, err := time.ParseDuration(value)
		if err != nil || duration < 0 {
			return usageError("invalid value %q for %s", value, name)
		}
		opts.MaxTime = duration
		return nil
	}
	if name == "--max-spend" {
		spend, err := strconv.ParseFloat(value, 64)
		if err != nil || spend < 0 || math.IsNaN(spend) || math.IsInf(spend, 0) {
			return usageError("invalid value %q for %s", value, name)
		}
		opts.MaxSpend = spend
		return nil
	}

	n, err := strconv.Atoi(value)
	if err != nil || n < 0 || (name != "--max-iterations" && n == 0) {
		return usageError("invalid value %q for %s", value, name)
	}
	switch name {
	case "--repeat":
		opts.Repeat = n
	case "--parallel":
		opts.Parallel = n
	case "--worst":
		opts.Worst = n
	case "--max-retries":
		opts.MaxRetries = n
	case "--max-iterations":
		opts.MaxIterations = n
	default:
		panic("cli: unhandled value flag " + name)
	}
	return nil
}

func usageError(format string, args ...any) error {
	return &UsageError{Message: fmt.Sprintf(format, args...)}
}
