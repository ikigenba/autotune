package app

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/ikigenba/agentkit"
	"github.com/ikigenba/autotune/internal/cli"
	"github.com/ikigenba/autotune/internal/config"
	"github.com/ikigenba/autotune/internal/folder"
	"github.com/ikigenba/autotune/internal/loop"
	"github.com/ikigenba/autotune/internal/scorer"
	"github.com/ikigenba/autotune/internal/workspace"
)

type Deps struct {
	Stdout, Stderr io.Writer
	Getenv         func(string) string
	Now            func() time.Time
	Home           string
	IsTTY          bool
	Version        string
	NewProvider    config.ProviderFactory
}

func Run(ctx context.Context, deps Deps, args []string) int {
	opts, err := cli.Parse(args)
	if err != nil {
		fmt.Fprintln(deps.Stderr, err)
		fmt.Fprint(deps.Stderr, cli.UsageText)
		return cli.Usage.ExitCode()
	}
	if opts.Help {
		fmt.Fprint(deps.Stdout, cli.UsageText)
		return cli.Done.ExitCode()
	}
	if opts.Version {
		fmt.Fprintln(deps.Stdout, deps.Version)
		return cli.Done.ExitCode()
	}
	if opts.Init {
		if err := folder.Init(opts.Folder); err != nil {
			fmt.Fprintln(deps.Stderr, err)
			return cli.Failed.ExitCode()
		}
		return cli.Done.ExitCode()
	}

	f, err := folder.Load(opts.Folder)
	if err != nil {
		fmt.Fprintln(deps.Stderr, err)
		return cli.Failed.ExitCode()
	}
	cfg, err := config.Resolve(f.ConfigRaw, opts.Config, deps.Getenv, deps.Home)
	if err != nil {
		fmt.Fprintln(deps.Stderr, err)
		return cli.Failed.ExitCode()
	}
	if err := cfg.PricingPrecheck(opts.MaxSpend); err != nil {
		fmt.Fprintln(deps.Stderr, err)
		return cli.Usage.ExitCode()
	}
	w, err := workspace.Create(f.Root, deps.Now())
	if err != nil {
		fmt.Fprintln(deps.Stderr, err)
		return cli.Failed.ExitCode()
	}
	if err := w.WriteConfigStamp(cfg); err != nil {
		fmt.Fprintln(deps.Stderr, err)
		return cli.Failed.ExitCode()
	}
	if deps.NewProvider == nil {
		fmt.Fprintln(deps.Stderr, "provider factory is not configured")
		return cli.Failed.ExitCode()
	}
	store := &healthStore{
		workspace: w,
		out:       deps.Stdout,
		tty:       deps.IsTTY,
		color:     deps.IsTTY && deps.Getenv("NO_COLOR") == "",
	}

	_, code := loop.Run(ctx, loop.Deps{
		RunnerConv: func(system string) (*agentkit.Conversation, error) {
			return deps.NewProvider(cfg.Runner, system)
		},
		ImproverConv: func(system string) (*agentkit.Conversation, error) {
			return deps.NewProvider(cfg.Improver, system)
		},
		Scorer:    scorer.New(f.ScorePath, f.Root),
		Out:       deps.Stdout,
		Err:       deps.Stderr,
		Now:       deps.Now,
		Workspace: store,
	}, f, cfg, loop.Options{
		Repeat:     opts.Repeat,
		Parallel:   opts.Parallel,
		MaxRetries: opts.MaxRetries,
		WorstK:     opts.Worst,
		Rails: loop.Rails{
			MaxIterations: opts.MaxIterations,
			MaxTime:       opts.MaxTime,
			MaxSpend:      opts.MaxSpend,
		},
	})
	return code
}
