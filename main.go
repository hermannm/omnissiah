package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"os"
	"slices"
	"strings"

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
	var empowerArgs empowerArgs

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

	if _, err := sbx.ResetPolicies(ctx); err != nil {
		return wrap.Error(err, "failed to reset sbx policies (needed to init network policy)")
	}
	if _, err := sbx.InitNetworkPolicy(ctx, sbx.NetworkPolicyDenyAll); err != nil {
		return wrap.Errorf(
			err,
			"failed to initialize '%s' sbx network policy",
			sbx.NetworkPolicyDenyAll,
		)
	}
	slog.InfoContext(ctx, "Initialized sbx network policy", "policy", sbx.NetworkPolicyDenyAll)

	if _, err := sbx.RestartDaemon(ctx); err != nil {
		return wrap.Error(err, "failed to restart sbx daemon")
	}
	slog.InfoContext(ctx, "Restarted sbx daemon to apply settings")

	return nil
}

type empowerArgs struct {
	projectName  string
	skipAuth     bool
	skipNetwork  bool
	skipProjects bool
}

func empower(ctx context.Context, args empowerArgs) error {
	cfg, err := config.Load()
	if err != nil {
		return wrap.Error(err, "failed to load Omnissiah config")
	}

	if args.projectName != "" {
		project, err := cfg.GetProject(args.projectName)
		if err != nil {
			return err
		}
		if err := bootstrapProject(ctx, project); err != nil {
			return wrap.Errorf(err, "failed to bootstrap project '%s'", args.projectName)
		}
		return nil
	}

	if !args.skipAuth {
		if err := authenticateModels(ctx, cfg); err != nil {
			return wrap.Error(err, "failed to authenticate models")
		}
	}

	if !args.skipNetwork {
		if err := allowNetworks(ctx, cfg); err != nil {
			return err
		}
	}

	if !args.skipProjects {
		for _, project := range cfg.Projects {
			if err := bootstrapProject(ctx, project); err != nil {
				return wrap.Errorf(err, "failed to bootstrap project '%s'", project.Name)
			}
		}
	}

	return nil
}

func bootstrapProject(ctx context.Context, project config.Project) error {
	templateName := project.TemplateName()

	shouldContinue, err := removeExistingTemplate(ctx, templateName)
	if err != nil {
		return err
	}
	if !shouldContinue {
		return nil
	}

	spiritName := project.Name + "-bootstrap"
	fmt.Printf("Waking machine spirit: %s\n", color.Red.On(spiritName))

	if err := sbx.RunDetached(
		ctx,
		spiritName,
		project.Model.Name.V3(),
		project.AbsolutePath,
		// No template when bootstrapping
		"",
		project.Kits,
		project.Model.Args,
	); err != nil {
		return err
	}

	for _, cmd := range project.Bootstrap {
		if err := runBootstrapCommand(ctx, cmd, spiritName); err != nil {
			return err
		}
	}

	// Sandbox must be stopped before saving its template
	if _, err := sbx.Stop(ctx, spiritName); err != nil {
		return wrap.ErrorWithAttrs(
			err,
			"failed to stop machine spirit before saving",
			"spiritName", spiritName,
		)
	}

	slog.InfoContext(ctx, fmt.Sprintf("Saving template %s...", color.Red.On(templateName)))
	if err := sbx.SaveTemplate(ctx, spiritName, templateName); err != nil {
		return wrap.ErrorWithAttrs(
			err,
			"failed to save template for project",
			"templateName", templateName,
		)
	}

	// TODO: Use errclose.Cleanup after RunDetached instead
	if _, err := sbx.Remove(ctx, spiritName, true); err != nil {
		return wrap.Errorf(err, "failed to kill machine spirit '%s' after bootstrap", spiritName)
	}

	slog.InfoContext(
		ctx,
		fmt.Sprintf("Successfully bootstrapped project %s", color.Red.On(project.Name)),
	)
	return nil
}

func runBootstrapCommand(
	ctx context.Context,
	cmd config.BootstrapCommand,
	spiritName string,
) error {
	switch {
	case cmd.Cmd != "":
		fmt.Print(color.Gray.On(fmt.Sprintf("> %s\n\n", cmd.Cmd)))

		if err := sbx.Exec(ctx, spiritName, cmd.Cmd); err != nil {
			return wrap.ErrorWithAttrs(err, "bootstrap command failed", "command", cmd.Cmd)
		}
	case cmd.Cp != nil:
		fmt.Print(
			color.Gray.On(
				fmt.Sprintf("> cp %s -> %s\n\n", cmd.Cp.From, cmd.Cp.ToAbsolute),
			),
		)

		if err := sbx.Copy(
			ctx,
			cmd.Cp.FromAbsolute,
			cmd.Cp.ToAbsolute,
			spiritName,
		); err != nil {
			return wrap.ErrorWithAttrs(
				err,
				"bootstrap cp command failed",
				"from", cmd.Cp.From,
				"to", cmd.Cp.To,
			)
		}
	}
	fmt.Println() // Newline after command, for better separation
	return nil
}

func removeExistingTemplate(
	ctx context.Context,
	templateName string,
) (shouldContinue bool, err error) {
	templates, err := sbx.ListTemplates(ctx)
	if err != nil {
		return false, err
	}
	exists := slices.ContainsFunc(
		templates,
		func(template sbx.Template) bool {
			// Local template names are actually stored in the "repository" field, as
			// "docker.io/library/<TEMPLATE_NAME>", while the "tag" field will be "latest" if not
			// specified
			return strings.Contains(template.Repository, templateName)
		},
	)
	// No existing template found with given tag: all good, continue
	if !exists {
		return true, nil
	}

	fmt.Printf(
		"Template %s already exists for project, rebuild it? (Y/n) ",
		color.Red.On(templateName),
	)
	shouldContinue, err = tui.PromptUserYesOrNo(new(true))
	if err != nil {
		return false, wrap.Error(err, "yes/no prompt failed")
	}
	if !shouldContinue {
		return false, nil
	}
	fmt.Println() // Newline for spacing

	if _, err := sbx.RemoveTemplate(ctx, templateName); err != nil {
		return false, wrap.Errorf(
			err,
			"failed to remove existing project template '%s'",
			templateName,
		)
	}

	return true, nil
}

func authenticateModels(ctx context.Context, cfg config.Config) error {
	var models []config.ModelName
	if cfg.DefaultModel != nil {
		models = append(models, cfg.DefaultModel.Name)
	}
	for _, project := range cfg.Projects {
		if project.Model != nil && !slices.Contains(models, project.Model.Name) {
			models = append(models, project.Model.Name)
		}
	}

ModelLoop:
	for _, model := range models {
		provider, ok := model.Provider()
		if !ok {
			slog.ErrorContext(
				ctx,
				fmt.Sprintf(
					"No provider configured for model '%s', please configure sbx directly",
					model,
				),
				"sbxDocs", "https://docs.docker.com/reference/cli/sbx/secret/set/",
			)
			continue
		}

		slog.InfoContext(
			ctx,
			fmt.Sprintf(
				"Authenticating model %s... (discovered by the tech priests of %s)",
				color.Red.On(model), color.Red.On(provider),
			),
		)

		if model.Matches(config.ModelClaude) {
			fmt.Print(
				`
  If you have a Claude subscription, you can instead run /login on your first prayer.
  This stores a session token on your host. Read sbx docs for more:
  https://docs.docker.com/ai/sandboxes/get-started/#authenticate-your-agent

  Do you still want to set an API token? (y/N) `,
			)
			yes, err := tui.PromptUserYesOrNo(new(false))
			if err != nil {
				return wrap.Error(err, "yes/no prompt failed")
			}
			if !yes {
				fmt.Println() // Newline for spacing
				continue ModelLoop
			}
		}

		if err := sbx.SetSecret(ctx, provider); err != nil {
			return wrap.Errorf(err, "failed to set secret for model provider '%s'", provider)
		}
	}

	slog.InfoContext(ctx, fmt.Sprintf("Authenticated all models configured in %s", cfg.Path.Short))
	return nil
}

func allowNetworks(ctx context.Context, cfg config.Config) error {
	for _, network := range cfg.AllowedNetworks {
		if _, err := sbx.AllowNetwork(ctx, network); err != nil {
			return wrap.ErrorWithAttrs(
				err,
				"failed to allow network in sbx policy",
				"network", network,
			)
		}
	}
	slog.InfoContext(
		ctx,
		"Configured allowed networks in sbx policy",
		"allowedNetworks", cfg.AllowedNetworks,
	)
	return nil
}

type prayArgs struct {
	name string
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

	if err := sbx.RunDetached(
		ctx,
		args.name,
		// Use V3 models for sbx kit compatibility
		project.Model.Name.V3(),
		project.AbsolutePath,
		project.FullTemplateTag(),
		project.Kits,
		project.Model.Args,
	); err != nil {
		return err
	}

	if err := sbx.RunExisting(ctx, args.name); err != nil {
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
