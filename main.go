package main

import (
	"context"
	"log/slog"
	"os"
	"slices"

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

	var awakenArgs awakenArgs
	var empowerArgs empowerArgs
	var prayArgs prayArgs

	//exhaustruct:ignore
	cmd := &cli.Command{
		Name:    "omnissiah",
		Usage:   "A thin, opinionated wrapper around Docker Sandboxes for praying to machine spirits (a.k.a. sandboxing AI agents)",
		Version: "0.1.0",
		Commands: []*cli.Command{
			{
				Name:  "awaken",
				Usage: "Initialize Omnissiah config and set sbx defaults",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:        "skip-settings",
						Usage:       "Skip default sbx settings",
						Destination: &awakenArgs.skipSettings,
					},
					&cli.BoolFlag{
						Name:        "skip-network",
						Usage:       "Skip sbx network policy initialization",
						Destination: &awakenArgs.skipNetwork,
					},
				},
				Action: func(ctx context.Context, _ *cli.Command) error {
					return awaken(ctx, awakenArgs)
				},
			},
			{
				Name:  "empower",
				Usage: "Post-config setup (model provider auth, network policies, project bootstrap)",
				Arguments: []cli.Argument{
					&cli.StringArg{
						Name:        "projectName",
						Required:    false,
						UsageText:   "[project-name] (pass this to only run bootstrap for the given project)",
						Destination: &empowerArgs.projectName,
					},
				},
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:        "skip-auth",
						Usage:       "Skip model provider authentication",
						Destination: &empowerArgs.skipAuth,
					},
					&cli.BoolFlag{
						Name:        "skip-network",
						Usage:       "Skip sbx network policy configuration",
						Destination: &empowerArgs.skipNetwork,
					},
					&cli.BoolFlag{
						Name:        "skip-projects",
						Usage:       "Skip project bootstrap",
						Destination: &empowerArgs.skipProjects,
					},
				},
				Action: func(ctx context.Context, _ *cli.Command) error {
					return empower(ctx, empowerArgs)
				},
			},
			{
				Name:    "pray",
				Aliases: []string{"p"},
				Usage:   "Start a new Omnissiah session in the current Git repo",
				Arguments: []cli.Argument{
					&cli.StringArg{
						Name:        "name",
						Required:    false,
						UsageText:   "[name] (optional - defaults to random 3-letter name)",
						Destination: &prayArgs.name,
					},
				},
				Action: func(ctx context.Context, _ *cli.Command) error {
					return pray(ctx, prayArgs)
				},
			},
			{
				Name:    "commune",
				Aliases: []string{"c"},
				Usage:   "Commune with existing machine spirits",
				Action: func(ctx context.Context, _ *cli.Command) error {
					return commune(ctx)
				},
			},
		},
		EnableShellCompletion: true,
		Authors:               []any{"Hermann Mørkrid"},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		// If error came from sbx, we don't want to log if its output was already forwarded, as that
		// will lead to duplicate error logs.
		sbxErr, ok := sbx.AsSBXError(err)
		if ok {
			if !sbxErr.OutputForwarded {
				slog.Error("The Omnissiah failed", errlog.Cause(err))
			}
			os.Exit(sbxErr.ExitCode)
		} else {
			slog.Error("The Omnissiah failed", errlog.Cause(err))
			os.Exit(1)
		}
	}
}

type sortArgs struct {
	runningFirst bool
	newestFirst  bool
}

func sortSpirits(machineSpirits []sbx.Sandbox, args sortArgs) {
	slices.SortFunc(
		machineSpirits,
		func(s1 sbx.Sandbox, s2 sbx.Sandbox) int {
			// Sort by status first
			if s1.Status != s2.Status {
				if s1.Status == sbx.StatusRunning {
					if args.runningFirst {
						return -1
					} else {
						return 1
					}
				}
				if s2.Status == sbx.StatusRunning {
					if args.runningFirst {
						return 1
					} else {
						return -1
					}
				}
			}

			// Then sort by created time
			createdAtCmp := s1.CreatedAt.Compare(s2.CreatedAt)
			// Sort is ascending, so we have to invert it to get newest first
			if args.newestFirst {
				return -createdAtCmp
			} else {
				return createdAtCmp
			}
		},
	)
}
