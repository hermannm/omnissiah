package main

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"hermannm.dev/wrap"

	"hermannm.dev/omnissiah/config"
	"hermannm.dev/omnissiah/internal/color"
	"hermannm.dev/omnissiah/sbx"
	"hermannm.dev/omnissiah/tui"
)

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
