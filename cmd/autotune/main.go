package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ikigenba/agentkit"
	"github.com/ikigenba/autotune/internal/app"
	"github.com/ikigenba/autotune/internal/config"
)

var version = "dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	home, _ := os.UserHomeDir()
	isTTY := false
	if info, err := os.Stdout.Stat(); err == nil {
		isTTY = info.Mode()&os.ModeCharDevice != 0
	}
	os.Exit(app.Run(ctx, app.Deps{
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
		Getenv:  os.Getenv,
		Now:     time.Now,
		Home:    home,
		IsTTY:   isTTY,
		Version: version,
		NewProvider: func(section config.Section, system string) (*agentkit.Conversation, error) {
			return section.Conversation(system)
		},
	}, os.Args[1:]))
}
