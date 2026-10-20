package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/urfave/cli/v3"
	"hermannm.dev/devlog"
	"hermannm.dev/devlog/errlog"
	"hermannm.dev/devlog/sloginit"
	"hermannm.dev/omnissiah/sbx"
)

func main() {
	sloginit.InitPrettyLogHandler(
		os.Stdout,
		&devlog.Options{TimeFormat: devlog.TimeFormatNone, RenameErrorAttrKey: "cause"},
	)

	cmd := &cli.Command{
		Name:    "omnissiah",
		Usage:   "A thin, opinionated wrapper around Docker Sandboxes for praying to machine spirits (a.k.a. sandboxing AI agents)",
		Version: "0.1.0",
		Commands: []*cli.Command{
			{
				Name:    "pray",
				Aliases: []string{"p"},
				Usage:   "Start a new Omnissiah session in the current Git repo",
				Action:  pray,
			},
		},
		EnableShellCompletion: true,
		Authors:               []any{"Hermann Mørkrid"},
	}
	if err := cmd.Run(context.Background(), os.Args); err != nil {
		// If sbx gave an exit error: use same exit code and don't log, as sbx likely already logged
		// Otherwise: log the error and exit with code 1
		exitErr, ok := sbx.AsExitError(err)
		if ok {
			os.Exit(exitErr.ExitCode())
		} else {
			slog.Error("The Omnissiah failed", errlog.Cause(err))
			os.Exit(1)
		}
	}
}

func pray(ctx context.Context, _ *cli.Command) error {
	if err := sbx.Run(ctx, "claude"); err != nil {
		return err
	}

	return nil
}
