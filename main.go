package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strconv"

	"github.com/urfave/cli/v3"
	"hermannm.dev/devlog"
	"hermannm.dev/devlog/errlog"
	"hermannm.dev/devlog/sloginit"
	"hermannm.dev/wrap"

	"hermannm.dev/omnissiah/config"
	"hermannm.dev/omnissiah/sbx"
)

func main() {
	sloginit.InitPrettyLogHandler(
		os.Stdout,
		&devlog.Options{TimeFormat: devlog.TimeFormatNone, RenameErrorAttrKey: "cause"},
	)

	//exhaustruct:ignore
	cmd := &cli.Command{
		Name:    "omnissiah",
		Usage:   "A thin, opinionated wrapper around Docker Sandboxes for praying to machine spirits (a.k.a. sandboxing AI agents)",
		Version: "0.1.0",
		Commands: []*cli.Command{
			{
				Name:   "awaken",
				Usage:  "Initialize Omnissiah config and set sbx defaults",
				Action: awaken,
			},
			{
				Name:      "pray",
				Aliases:   []string{"p"},
				Usage:     "Start a new Omnissiah session in the current Git repo",
				Action:    pray,
				ArgsUsage: "[name] (optional - defaults to incrementing numeric name)",
			},
			{
				Name:    "cortex",
				Aliases: []string{"cc"},
				Usage:   "Open the Cybernetica Cortex: the machine spirit control center",
				Action:  cortex,
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

func awaken(ctx context.Context, _ *cli.Command) error {
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

func pray(ctx context.Context, cmd *cli.Command) error {
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

	name := cmd.Args().First()
	if name == "" {
		// If no name is given: Set numeric name (0, 1, 2...)
		name = getNumericNameForMachineSpirit(existingSpirits)
	} else {
		// If name is given: check that it's unique (otherwise sbx reattaches to existing sandbox,
		// which we reserve for the `commune` command)
		if slices.ContainsFunc(
			existingSpirits,
			func(spirit sbx.Sandbox) bool {
				return spirit.Name == name
			},
		) {
			return fmt.Errorf("machine spirit with name '%s' already exists", name)
		}
	}

	if err := sbx.Run(ctx, name, project.Model, project.Path); err != nil {
		return err
	}

	return nil
}

func getNumericNameForMachineSpirit(existingSpirits []sbx.Sandbox) string {
	existingNumericNames := make([]int, 0, len(existingSpirits))

	for _, spirit := range existingSpirits {
		numericName, err := strconv.Atoi(spirit.Name)
		if err == nil {
			existingNumericNames = append(existingNumericNames, numericName)
		}
	}

	switch len(existingNumericNames) {
	case 0:
		return formatNumericName(0)
	case 1:
		if existingNumericNames[0] == 0 {
			return formatNumericName(1)
		} else {
			return formatNumericName(0)
		}
	default:
		var previousName int
		for i, name := range existingNumericNames {
			if i == 0 {
				previousName = name
				continue
			}

			if name-previousName > 1 {
				return formatNumericName(previousName + 1)
			}
			previousName = name
		}
		return formatNumericName(previousName + 1)
	}
}

func formatNumericName(name int) string {
	nameString := strconv.Itoa(name)
	if len(nameString) == 1 {
		return "0" + nameString
	} else {
		return nameString
	}
}

func cortex(ctx context.Context, _ *cli.Command) error {
	return sbx.TerminalUI(ctx)
}
