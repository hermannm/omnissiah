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

	"hermannm.dev/omnissiah/config"
	"hermannm.dev/omnissiah/internal/color"
	"hermannm.dev/omnissiah/sbx"
	"hermannm.dev/omnissiah/tui"
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
				Name:  "empower",
				Usage: "Authenticate STC (AI model) providers",
				Action: func(ctx context.Context, _ *cli.Command) error {
					return empower(ctx)
				},
			},
			{
				Name:    "pray",
				Aliases: []string{"p", "pp"},
				Usage:   "Start a new Omnissiah session in the current Git repo",
				Arguments: []cli.Argument{
					&cli.StringArg{
						Name:        "name",
						Required:    false,
						UsageText:   "[name] (optional - defaults to random 3-letter name)",
						Destination: &prayArgs.name,
					},
				},
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:        "detach",
						Aliases:     []string{"d"},
						Usage:       "Detached mode: Leave the machine spirit running in the background",
						Destination: &prayArgs.detach,
					},
					&cli.BoolFlag{
						Name:        "persistent",
						Aliases:     []string{"p"},
						Usage:       "Pray interactively first, leave spirit awake on exit ('pp' alias sets this)",
						Destination: &prayArgs.persistent,
					},
				},
				Action: func(ctx context.Context, _ *cli.Command) error {
					if len(os.Args) >= 2 && os.Args[1] == "pp" {
						prayArgs.persistent = true
					}
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
		if _, err := sbx.SetSetting(ctx, setting, value); err != nil {
			return wrap.ErrorWithAttrs(
				err,
				"failed to set sbx setting",
				"setting", setting,
				"value", value,
			)
		}
	}
	slog.InfoContext(ctx, "Configured sbx default settings", "settings", sbxSettings)

	if _, err := sbx.RestartDaemon(ctx); err != nil {
		return wrap.Error(err, "failed to restart sbx daemon")
	}
	slog.InfoContext(ctx, "Restarted sbx daemon to apply settings")

	return nil
}

func empower(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return wrap.Error(err, "failed to load Omnissiah config")
	}

	var stcs []string
	if cfg.DefaultSTC != nil {
		stcs = append(stcs, cfg.DefaultSTC.Model)
	}
	for _, project := range cfg.Projects {
		if project.STC != nil && !slices.Contains(stcs, project.STC.Model) {
			stcs = append(stcs, project.STC.Model)
		}
	}

	// See "Available services" here: https://docs.docker.com/reference/cli/sbx/secret/set/
	stcProviderMap := map[string]string{
		config.STCClaude:  "anthropic",
		config.STCCodex:   "openai",
		config.STCCopilot: "github",
		config.STCGemini:  "google",
	}
STCLoop:
	for _, stc := range stcs {
		provider, ok := stcProviderMap[stc]
		if !ok {
			slog.ErrorContext(
				ctx,
				fmt.Sprintf(
					"No provider configured for STC '%s', please configure sbx directly",
					stc,
				),
				"sbxDocs", "https://docs.docker.com/reference/cli/sbx/secret/set/",
			)
			continue
		}

		slog.InfoContext(
			ctx,
			fmt.Sprintf(
				"Loading STC %s... (discovered by the tech priests of %s)",
				color.Red.On(stc), color.Red.On(provider),
			),
		)

		if stc == config.STCClaude {
			fmt.Print(
				`
  If you have a Claude subscription, you can instead run /login on your first prayer.
  This stores a session token on your host. Read sbx docs for more:
  https://docs.docker.com/ai/sandboxes/get-started/#authenticate-your-agent

  Do you still want to set an API token? (y/n) `,
			)
			yes, err := tui.PromptUserYesOrNo()
			if err != nil {
				return wrap.Error(err, "yes/no prompt failed")
			}
			if !yes {
				fmt.Println() // Newline for spacing
				continue STCLoop
			}
		}

		if err := sbx.SetSecret(ctx, provider); err != nil {
			return wrap.Errorf(err, "failed to set secret for STC provider '%s'", provider)
		}
	}

	slog.InfoContext(ctx, fmt.Sprintf("Loaded all STCs configured in %s", cfg.Path.Short))
	return nil
}

type prayArgs struct {
	name       string
	detach     bool
	persistent bool
}

func pray(ctx context.Context, args prayArgs) error {
	cfg, err := config.Load()
	if err != nil {
		return wrap.Error(err, "failed to load Omnissiah config")
	}

	project, err := cfg.GetCurrentProject()
	if err != nil {
		return err
	}

	existingSpirits, err := sbx.List(ctx)
	if err != nil {
		return wrap.Error(err, "failed to list machine spirits for giving name to the new one")
	}

	if args.name == "" {
		// If no name is given: Get random name from [machineSpiritNames]
		args.name, err = getRandomNameForMachineSpirit(existingSpirits, defaultRand())
		if err != nil {
			return wrap.Error(err, "failed to get random name for machine spirit")
		}
	} else {
		// If name is given: check that it's unique (otherwise sbx reattaches to existing sandbox,
		// which we reserve for the `commune` command)
		if slices.ContainsFunc(
			existingSpirits,
			func(spirit sbx.Sandbox) bool {
				return spirit.Name == args.name
			},
		) {
			return fmt.Errorf("machine spirit with name '%s' already exists", args.name)
		}
	}

	fmt.Printf("Waking machine spirit: %s\n", color.Red.On(args.name))

	if err := sbx.Run(
		ctx,
		args.name,
		project.STC.Model,
		project.Path,
		// For persistent mode, we start in detached mode and then join interactively after
		args.detach || args.persistent,
		project.STC.Args,
	); err != nil {
		return err
	}

	if args.persistent {
		if err := sbx.RunExisting(ctx, args.name); err != nil {
			return err
		}
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

func commune(ctx context.Context) error {
	machineSpirits, err := sbx.List(ctx)
	if err != nil {
		return wrap.Error(err, "failed to list machine spirits")
	}

	sortSpirits(machineSpirits, sortArgs{runningFirst: true, newestFirst: true})

	if err := tui.RenderMachineSpiritPicker(
		ctx,
		machineSpirits,
		[]tui.PickerAction{
			pickerActionCommune,
			pickerActionWake,
			pickerActionSleep,
			pickerActionKill,
		},
		pickerActionCommune.Name,
	); err != nil {
		return err
	}
	return nil
}

var (
	pickerActionCommune = tui.PickerAction{
		Name: "commune",
		Key:  "c",
		Action: func(ctx context.Context, spirit sbx.Sandbox) (sbx.Sandbox, error) {
			if err := sbx.RunExisting(ctx, spirit.Name); err != nil {
				return sbx.Sandbox{}, err
			}
			spirit.Status = sbx.StatusRunning
			return spirit, nil
		},
	}
	pickerActionWake = tui.PickerAction{
		Name:       "wake",
		Key:        "w",
		Background: "Waking...",
		Action: func(ctx context.Context, spirit sbx.Sandbox) (sbx.Sandbox, error) {
			if _, err := sbx.RunExistingDetached(ctx, spirit.Name); err != nil {
				return sbx.Sandbox{}, err
			}
			spirit.Status = sbx.StatusRunning
			return spirit, nil
		},
	}
	pickerActionSleep = tui.PickerAction{
		Name:       "sleep",
		Key:        "s",
		Background: "Putting to sleep...",
		Action: func(ctx context.Context, spirit sbx.Sandbox) (sbx.Sandbox, error) {
			if _, err := sbx.Stop(ctx, spirit.Name); err != nil {
				return sbx.Sandbox{}, err
			}
			spirit.Status = sbx.StatusStopped
			return spirit, nil
		},
	}
	pickerActionKill = tui.PickerAction{
		Name:                "kill",
		Key:                 "k",
		Background:          "Killing...",
		RequireConfirmation: true,
		Action: func(ctx context.Context, spirit sbx.Sandbox) (sbx.Sandbox, error) {
			if _, err := sbx.Remove(ctx, spirit.Name, true); err != nil {
				return sbx.Sandbox{}, err
			}
			return spirit, tui.ErrRemovePickerItem
		},
	}
)

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

func cortex(ctx context.Context) error {
	return sbx.TerminalUI(ctx)
}

func exterminatus(ctx context.Context) error {
	fmt.Printf("Commencing %s\n", color.Red.On("EXTERMINATUS"))

	return sbx.Prune(ctx)
}
