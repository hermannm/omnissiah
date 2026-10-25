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

func Run(
	ctx context.Context,
	sandboxName string,
	model string,
	path string,
	detached bool,
	agentArgs []string,
) error {
	sbxArgs := []string{
		"run",
		// Always run in clone mode, so the agent doesn't edit our own files:
		// https://docs.docker.com/ai/sandboxes/usage/#clone-mode
		"--clone",
		"--name",
		sandboxName,
		model,
		path,
	}
	if detached {
		// https://docs.docker.com/ai/sandboxes/usage/#keep-a-sandbox-running-in-the-background
		sbxArgs = append(sbxArgs, "--detach")
	}
	if len(agentArgs) != 0 {
		// sbx docs: "Pass agent arguments after the "--" separator"
		// https://docs.docker.com/reference/cli/sbx/run/
		sbxArgs = append(sbxArgs, "--")
		sbxArgs = append(sbxArgs, agentArgs...)
	}
	return sbx(ctx, sbxArgs...)
}

func RunExisting(ctx context.Context, sandboxName string) error {
	return sbx(ctx, "run", "--name", sandboxName)
}

func RunExistingDetached(ctx context.Context, sandboxName string) (stdout []byte, err error) {
	return sbxOutput(ctx, "run", "--detach", "--name", sandboxName)
}

func Stop(ctx context.Context, sandboxName string) (stdout []byte, err error) {
	return sbxOutput(ctx, "stop", sandboxName)
}

func Remove(ctx context.Context, sandboxName string, force bool) (stdout []byte, err error) {
	args := []string{"rm", sandboxName}
	if force {
		args = append(args, "--force")
	}
	return sbxOutput(ctx, args...)
}

func Prune(ctx context.Context) error {
	return sbx(ctx, "prune")
}

type Setting string

// https://docs.docker.com/ai/sandboxes/configuration/settings/
const (
	SettingSSHAgentForwardingEnabled Setting = "ssh.agentForwardingEnabled"
	SettingSkillsDefaultMode         Setting = "skills.defaultMode"
	SettingDiagnosticsAutoUpload     Setting = "diagnostics.autoUpload"
)

func SetSetting(ctx context.Context, setting Setting, value string) (stdout []byte, err error) {
	return sbxOutput(ctx, "settings", "set", string(setting), value)
}

func RestartDaemon(ctx context.Context) (stdout []byte, err error) {
	return sbxOutput(ctx, "daemon", "restart")
}

func TerminalUI(ctx context.Context) error {
	return sbx(ctx, "tui")
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
		return newError(err, args, true)
	}
	return nil
}

func sbxOutput(ctx context.Context, args ...string) (stdout []byte, err error) {
	cmd := exec.CommandContext(ctx, "sbx", args...)

	stdout, err = cmd.Output()
	if err != nil {
		return nil, newError(err, args, false)
	}
	return stdout, nil
}

type Error struct {
	args     []string
	cause    error
	ExitCode int
	// True if sbx's output was forwarded to Omnissiah's stdout (we use this to determine whether
	// we should log an error, or assume that sbx has done it for us).
	OutputForwarded bool
}

func newError(cause error, args []string, outputForwarded bool) Error {
	exitCode := 1
	if exitErr, ok := errors.AsType[*exec.ExitError](cause); ok {
		exitCode = exitErr.ExitCode()

		stderr := string(exitErr.Stderr)
		if len(stderr) != 0 {
			if stderr[len(stderr)-1] == '\n' {
				stderr = stderr[0 : len(stderr)-1]
			}

			// Replace "exit status 1" error with more useful stderr error
			cause = errors.New(stderr)
		}
	}

	return Error{
		args:            args,
		cause:           cause,
		ExitCode:        exitCode,
		OutputForwarded: outputForwarded,
	}
}

func (err Error) Error() string {
	return fmt.Sprintf("'sbx %s' failed: %v", strings.Join(err.args, " "), err.cause)
}

func (err Error) Unwrap() error {
	return err.cause
}

func AsSBXError(err error) (sbxErr Error, ok bool) {
	return errors.AsType[Error](err)
}
