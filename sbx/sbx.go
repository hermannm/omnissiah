package sbx

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"hermannm.dev/wrap"
)

func Run(ctx context.Context, name string, model string, path string, agentArgs []string) error {
	sbxArgs := []string{
		"run",
		// Always run in clone mode, so the agent doesn't edit our own files:
		// https://docs.docker.com/ai/sandboxes/usage/#clone-mode
		"--clone",
		"--name",
		name,
		model,
		path,
	}
	if len(agentArgs) != 0 {
		// sbx docs: "Pass agent arguments after the "--" separator"
		// https://docs.docker.com/reference/cli/sbx/run/
		sbxArgs = append(sbxArgs, "--")
		sbxArgs = append(sbxArgs, agentArgs...)
	}
	return sbx(ctx, sbxArgs...)
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

func Prune(ctx context.Context) error {
	return sbx(ctx, "prune")
}

func SetSecret(ctx context.Context, provider string) error {
	return sbx(ctx, "secret", "set", provider)
}

func List(ctx context.Context) ([]Sandbox, error) {
	output, err := sbxOutput(ctx, "ls", "--json")
	if err != nil {
		return nil, err
	}

	var sandboxes []Sandbox
	if err := json.Unmarshal(output, &sandboxes); err != nil {
		return nil, wrap.ErrorWithAttrs(
			err,
			"failed to parse 'sbx ls' output as JSON",
			"output", string(output),
		)
	}

	return sandboxes, nil
}

type Sandbox struct {
	Name       string    `json:"name"`
	ID         string    `json:"id"`
	Agent      string    `json:"agent"`
	Status     Status    `json:"status"`
	LastUsedAt time.Time `json:"last_used_at"`
	Workspaces []string  `json:"workspaces"`
	CreatedAt  time.Time `json:"created_at"`
	// Only included when [Sandbox.Status] == [StatusRunning].
	Ports []struct {
		HostIP      string `json:"host_ip"`
		HostPort    int    `json:"host_port"`
		SandboxPort int    `json:"sandbox_port"`
		Protocol    string `json:"protocol"`
	} `json:"ports,omitempty"`
}

type Status string

const (
	StatusRunning Status = "running"
	StatusStopped Status = "stopped"
)

func ValidateStatus(status string) (Status, error) {
	s := Status(status)
	switch s {
	case StatusRunning, StatusStopped:
		return s, nil
	default:
		return s, fmt.Errorf("unknown sbx status value '%s'", status)
	}
}

// UnmarshalJSON validates [Status] values on unmarshaling.
func (status *Status) UnmarshalJSON(bytes []byte) (err error) {
	var s string
	if err := json.Unmarshal(bytes, &s); err != nil {
		return err
	}
	*status, err = ValidateStatus(s)
	return err
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

func sbxOutput(ctx context.Context, args ...string) (stdout []byte, err error) {
	cmd := exec.CommandContext(ctx, "sbx", args...)

	stdout, err = cmd.Output()
	if err != nil {
		return nil, wrap.Errorf(err, "'sbx %s' failed", strings.Join(args, " "))
	}
	return stdout, nil
}

func AsExitError(err error) (*exec.ExitError, bool) {
	return errors.AsType[*exec.ExitError](err)
}
