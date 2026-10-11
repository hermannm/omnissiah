package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/urfave/cli/v3"
	"hermannm.dev/devlog"
	"hermannm.dev/devlog/errlog"
	"hermannm.dev/devlog/sloginit"
)

func main() {
	sloginit.InitPrettyLogHandler(
		os.Stdout,
		&devlog.Options{TimeFormat: devlog.TimeFormatNone, RenameErrorAttrKey: "cause"},
	)

	cmd := &cli.Command{
		Name:           "omnissiah",
		Usage:          "A thin, opinionated wrapper around Docker Sandboxes for sandboxing AI agents",
		Version:        "0.1.0",
		DefaultCommand: "pray",
		Commands: []*cli.Command{
			{
				Name:    "pray",
				Aliases: []string{"p"},
				Usage:   "Start a new Omnissiah session",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					slog.InfoContext(ctx, "Praying...")
					return nil
				},
			},
		},
		EnableShellCompletion: true,
		Authors:               []any{"Hermann Mørkrid"},
	}
	if err := cmd.Run(context.Background(), os.Args); err != nil {
		slog.Error("The Omnissiah failed", errlog.Cause(err))
		os.Exit(1)
	}
}
