package sbx

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"

	"hermannm.dev/wrap"
)

func Run(ctx context.Context, model string, path string) error {
	// Always run in clone mode, so the agent doesn't edit our own files:
	// https://docs.docker.com/ai/sandboxes/usage/#clone-mode
	return sbx(ctx, "run", "--clone", model, path)
}

type Setting string

// https://docs.docker.com/ai/sandboxes/configuration/settings/
const (
	SettingSSHAgentForwardingEnabled Setting = "ssh.agentForwardingEnabled"
	SettingSkillsDefaultMode         Setting = "skills.defaultMode"
	SettingDiagnosticsAutoUpload     Setting = "diagnostics.autoUpload"
)

func SetSetting(ctx context.Context, setting Setting, value string) error {
	return sbx(ctx, "settings", "set", string(setting), value)
}

func RestartDaemon(ctx context.Context) error {
	return sbx(ctx, "daemon", "restart")
}

func TerminalUI(ctx context.Context) error {
	return sbx(ctx, "tui")
}

func sbx(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "sbx", args...)

	// Forward all input/output to/from sbx
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return wrap.Errorf(err, "'sbx %s' failed", strings.Join(args, " "))
	}
	return nil
}

func AsExitError(err error) (*exec.ExitError, bool) {
	return errors.AsType[*exec.ExitError](err)
}
