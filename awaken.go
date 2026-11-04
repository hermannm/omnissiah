package main

import (
	"context"
	"log/slog"

	"hermannm.dev/wrap"

	"hermannm.dev/omnissiah/config"
	"hermannm.dev/omnissiah/sbx"
)

type awakenArgs struct {
	skipSettings bool
	skipNetwork  bool
}

func awaken(ctx context.Context, args awakenArgs) error {
	if err := config.Initialize(); err != nil {
		return wrap.Error(err, "failed to initialize Omnissiah config")
	}

	if !args.skipSettings {
		if err := setDefaultSbxSettings(ctx); err != nil {
			return err
		}
	}

	if !args.skipNetwork {
		if err := initSbxNetworkPolicy(ctx); err != nil {
			return err
		}
	}

	if _, err := sbx.RestartDaemon(ctx); err != nil {
		return wrap.Error(err, "failed to restart sbx daemon")
	}
	slog.InfoContext(ctx, "Restarted sbx daemon to apply settings")

	return nil
}

func setDefaultSbxSettings(ctx context.Context) error {
	sbxSettings := map[sbx.Setting]string{
		// We don't want the agent to inherit our SSH privileges (for Git operations, use scoped
		// tokens with HTTP instead)
		sbx.SettingSSHAgentForwardingEnabled: "false",
		// Ensure that the agent can never write to shared skill files
		sbx.SettingSkillsDefaultMode: "readonly",
		// Disable sending diagnostics, since "collected logs can still contain user content":
		// https://docs.docker.com/ai/sandboxes/troubleshooting/#enable-automatic-diagnostics-uploads
		sbx.SettingDiagnosticsAutoUpload: "false",
		// Allow us to copy text from the sandbox
		sbx.SettingClipboardTextCopy: "true",
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
	return nil
}

func initSbxNetworkPolicy(ctx context.Context) error {
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
	return nil
}
