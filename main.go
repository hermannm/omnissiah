package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"os"
	"slices"

	"github.com/urfave/cli/v3"
	"hermannm.dev/devlog"
	"hermannm.dev/devlog/errlog"
	"hermannm.dev/devlog/sloginit"
	"hermannm.dev/wrap"

	"hermannm.dev/omnissiah/internal/color"

	"hermannm.dev/omnissiah/config"
	"hermannm.dev/omnissiah/sbx"
)

func main() {
	sloginit.InitPrettyLogHandler(
		os.Stdout,
		&devlog.Options{TimeFormat: devlog.TimeFormatNone, RenameErrorAttrKey: "cause"},
	)

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
				Action: func(ctx context.Context, _ *cli.Command) error {
					return awaken(ctx)
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
						Destination: &prayArgs.Name,
					},
				},
				Action: func(ctx context.Context, _ *cli.Command) error {
					return pray(ctx, prayArgs)
				},
			},
			{
				Name:    "cortex",
				Aliases: []string{"cc"},
				Usage:   "Open the Cybernetica Cortex: the machine spirit control center",
				Action: func(ctx context.Context, _ *cli.Command) error {
					return cortex(ctx)
				},
			},
			{
				Name:    "exterminatus",
				Aliases: []string{"e"},
				Usage:   "Commence Exterminatus on all machine spirits, killing them",
				Action: func(ctx context.Context, _ *cli.Command) error {
					return exterminatus(ctx)
				},
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

func awaken(ctx context.Context) error {
	if err := config.Initialize(); err != nil {
		return wrap.Error(err, "failed to initialize Omnissiah config")
	}

	sbxSettings := map[sbx.Setting]string{
		// We don't want the agent to inherit our SSH privileges (for Git operations, use scoped
		// tokens with HTTP instead)
		sbx.SettingSSHAgentForwardingEnabled: "false",
		// Ensure that the agent can never write to shared skill files
		sbx.SettingSkillsDefaultMode: "readonly",
		// Disable sending diagnostics, since "collected logs can still contain user content":
		// https://docs.docker.com/ai/sandboxes/troubleshooting/#enable-automatic-diagnostics-uploads
		sbx.SettingDiagnosticsAutoUpload: "false",
	}
	for setting, value := range sbxSettings {
		if err := sbx.SetSetting(ctx, setting, value); err != nil {
			return wrap.ErrorWithAttrs(
				err,
				"failed to set sbx setting",
				"setting", setting,
				"value", value,
			)
		}
	}
	slog.InfoContext(ctx, "Configured sbx default settings", "settings", sbxSettings)

	if err := sbx.RestartDaemon(ctx); err != nil {
		return wrap.Error(err, "failed to restart sbx daemon")
	}
	slog.InfoContext(ctx, "Restarted sbx daemon to apply settings")

	return nil
}

type prayArgs struct {
	Name string
}

func pray(ctx context.Context, args prayArgs) error {
	cfg, err := config.Load()
	if err != nil {
		return wrap.Error(err, "failed to load Omnissiah config")
	}

	project, err := cfg.GetCurrentProject()
	if err != nil {
		return wrap.Error(err, "failed to get current project")
	}

	existingSpirits, err := sbx.List(ctx)
	if err != nil {
		return wrap.Error(err, "failed to list machine spirits for giving name to the new one")
	}

	if args.Name == "" {
		// If no name is given: Get random name from [machineSpiritNames]
		args.Name, err = getRandomNameForMachineSpirit(existingSpirits, defaultRand())
		if err != nil {
			return wrap.Error(err, "failed to get random name for machine spirit")
		}
	} else {
		// If name is given: check that it's unique (otherwise sbx reattaches to existing sandbox,
		// which we reserve for the `commune` command)
		if slices.ContainsFunc(
			existingSpirits,
			func(spirit sbx.Sandbox) bool {
				return spirit.Name == args.Name
			},
		) {
			return fmt.Errorf("machine spirit with name '%s' already exists", args.Name)
		}
	}

	fmt.Printf("Praying to %v...\n", color.Red.On(args.Name)) //nolint:forbidigo

	if err := sbx.Run(ctx, args.Name, project.Model, project.Path); err != nil {
		return err
	}

	return nil
}

func getRandomNameForMachineSpirit(existingSpirits []sbx.Sandbox, rng *rand.Rand) (string, error) {
	availableNames := make([]string, 0, len(machineSpiritNames))

	for _, name := range machineSpiritNames {
		if !slices.ContainsFunc(
			existingSpirits,
			func(spirit sbx.Sandbox) bool {
				return spirit.Name == name
			},
		) {
			availableNames = append(availableNames, name)
		}
	}

	if len(availableNames) == 0 {
		return "", errors.New(
			"all random names taken - either kill an existing machine spirit, or set name explicitly",
		)
	}

	randomIndex := rng.IntN(len(availableNames))
	return availableNames[randomIndex], nil
}

func defaultRand() *rand.Rand {
	// #nosec G404 - Weak randomness is sufficient here
	return rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
}

var machineSpiritNames = []string{
	"bax",
	"bix",
	"dax",
	"fex",
	"gix",
	"hax",
	"jax",
	"kex",
	"lux",
	"mox",
	"nux",
	"pex",
	"rax",
	"tox",
	"vox",
	"zyx",
}

func cortex(ctx context.Context) error {
	return sbx.TerminalUI(ctx)
}

func exterminatus(ctx context.Context) error {
	return sbx.Prune(ctx)
}
