package sbx

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"hermannm.dev/wrap"

	"hermannm.dev/omnissiah/config"
)

// https://docs.docker.com/reference/cli/sbx/run/
func RunDetached(
	ctx context.Context,
	sandboxName string,
	model config.Model,
	path string,
	// Optional (blank when bootstrapping initial template)
	templateTag string,
	kits []config.Kit,
) error {
	sbxArgs := []string{
		"run",
		// Always run in clone mode, so the agent doesn't edit our own files:
		// https://docs.docker.com/ai/sandboxes/usage/#clone-mode
		"--clone",
		// We create sandboxes by running them in detached mode, and then re-attaching after.
		// This is to:
		// 1) Keep it alive even when we exit the sandbox (which we want, to let tasks finish)
		// 2) Be able to run setup commands before attaching
		// https://docs.docker.com/ai/sandboxes/usage/#keep-a-sandbox-running-in-the-background
		"--detach",
		"--name",
		sandboxName,
		// Use V3 models for sbx kit compatibility
		model.Name.V3(),
		path,
	}
	if templateTag != "" {
		// We only run local templates, so we never want to pull images
		sbxArgs = append(sbxArgs, "--pull", "never")
		sbxArgs = append(sbxArgs, "--template", templateTag)
	}

	// Models are kits in sbx, so we can pass kit args to the model kit.
	// For example, the built-in claude kit takes a 'model' argument - see 'args:' here:
	// https://hub.docker.com/layers/sbx/claude/latest/images/latest/sbx-kit-spec
	for _, modelKitArg := range model.KitArgs {
		sbxArgs = append(sbxArgs, "--kit-arg", model.ShortKitName()+"."+modelKitArg)
	}

	// sbx docs on mixin kits:
	// https://docs.docker.com/ai/sandboxes/customize/use-kits/#add-mixins
	for _, kit := range kits {
		sbxArgs = append(sbxArgs, "--kit", kit.AbsolutePath)
		name := kit.ShortName()
		// https://docs.docker.com/ai/sandboxes/customize/use-kits/#pass-arguments-to-kits
		for _, arg := range kit.Args {
			sbxArgs = append(sbxArgs, "--kit-arg", name+"."+arg)
		}
	}

	if len(model.Args) != 0 {
		// sbx docs: "Pass agent arguments after the "--" separator"
		// https://docs.docker.com/reference/cli/sbx/run/
		sbxArgs = append(sbxArgs, "--")
		sbxArgs = append(sbxArgs, model.Args...)
	}

	return sbx(ctx, sbxArgs...)
}

// https://docs.docker.com/reference/cli/sbx/run/
func RunExisting(ctx context.Context, sandboxName string) error {
	return sbx(ctx, "run", "--name", sandboxName)
}

// https://docs.docker.com/reference/cli/sbx/run/
func RunExistingDetached(ctx context.Context, sandboxName string) (stdout []byte, err error) {
	return sbxOutput(ctx, "run", "--detach", "--name", sandboxName)
}

// https://docs.docker.com/reference/cli/sbx/exec/
func Exec(ctx context.Context, sandboxName string, command string) error {
	sbxArgs := []string{"exec", sandboxName}
	commandArgs := strings.Split(command, " ")
	sbxArgs = append(sbxArgs, commandArgs...)
	return sbx(ctx, sbxArgs...)
}

// https://docs.docker.com/reference/cli/sbx/cp/
func Copy(
	ctx context.Context,
	fromHostPath string,
	toSandboxPath string,
	sandboxName string,
) error {
	return sbx(ctx, "cp", fromHostPath, sandboxName+":"+toSandboxPath)
}

// https://docs.docker.com/reference/cli/sbx/stop/
func Stop(ctx context.Context, sandboxName string) (stdout []byte, err error) {
	return sbxOutput(ctx, "stop", sandboxName)
}

// https://docs.docker.com/reference/cli/sbx/rm/
func Remove(ctx context.Context, sandboxName string) (stdout []byte, err error) {
	return sbxOutput(ctx, "rm", "--force", sandboxName)
}

// https://docs.docker.com/reference/cli/sbx/rm/
func RemoveAll(ctx context.Context) (stdout []byte, err error) {
	return sbxOutput(ctx, "rm", "--all", "--force")
}

type Setting string

// https://docs.docker.com/ai/sandboxes/configuration/settings/
const (
	SettingSSHAgentForwardingEnabled Setting = "ssh.agentForwardingEnabled"
	SettingSkillsDefaultMode         Setting = "skills.defaultMode"
	SettingDiagnosticsAutoUpload     Setting = "diagnostics.autoUpload"
	SettingClipboardTextCopy         Setting = "clipboard.textCopy"
)

// https://docs.docker.com/reference/cli/sbx/settings/set/
func SetSetting(ctx context.Context, setting Setting, value string) (stdout []byte, err error) {
	return sbxOutput(ctx, "settings", "set", string(setting), value)
}

type NetworkPolicy string

// https://docs.docker.com/reference/cli/sbx/policy/init/
//
//goland:noinspection GoUnusedConst
const (
	NetworkPolicyAllowAll NetworkPolicy = "allow-all"
	NetworkPolicyBalanced NetworkPolicy = "balanced"
	NetworkPolicyDenyAll  NetworkPolicy = "deny-all"
)

// https://docs.docker.com/reference/cli/sbx/policy/init/
func InitNetworkPolicy(ctx context.Context, policy NetworkPolicy) (stdout []byte, err error) {
	return sbxOutput(ctx, "policy", "init", string(policy))
}

// https://docs.docker.com/reference/cli/sbx/policy/reset/
func ResetPolicies(ctx context.Context) (stdout []byte, err error) {
	return sbxOutput(ctx, "policy", "reset", "--force")
}

// https://docs.docker.com/reference/cli/sbx/policy/allow/network/
func AllowNetwork(ctx context.Context, network config.Network) (stdout []byte, err error) {
	args := []string{"policy", "allow", "network", network.Host}

	if len(network.Methods) != 0 {
		// --method docs: "Restrict the rule to these HTTP methods (comma-separated)"
		args = append(args, "--method", strings.Join(network.Methods, ","))
	}

	return sbxOutput(ctx, args...)
}

// https://docs.docker.com/reference/cli/sbx/daemon/restart/
func RestartDaemon(ctx context.Context) (stdout []byte, err error) {
	return sbxOutput(ctx, "daemon", "restart")
}

// https://docs.docker.com/reference/cli/sbx/secret/set/
func SetSecret(ctx context.Context, provider string) error {
	return sbx(ctx, "secret", "set", provider)
}

// https://docs.docker.com/reference/cli/sbx/ls/
func List(ctx context.Context) ([]Sandbox, error) {
	return sbxJSONOutput[[]Sandbox](ctx, "ls", "--json")
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

func validateStatus(status string) (Status, error) {
	s := Status(status)
	switch s {
	case StatusRunning, StatusStopped:
		return s, nil
	default:
		return "", fmt.Errorf("unknown sbx status value '%s'", status)
	}
}

// UnmarshalJSON validates [Status] values on unmarshaling.
func (status *Status) UnmarshalJSON(bytes []byte) (err error) {
	var s string
	if err := json.Unmarshal(bytes, &s); err != nil {
		return err
	}
	*status, err = validateStatus(s)
	return err
}

// https://docs.docker.com/reference/cli/sbx/template/ls/
func ListTemplates(ctx context.Context) ([]Template, error) {
	return sbxJSONOutput[[]Template](ctx, "template", "ls", "--json")
}

type Template struct {
	ID         string `json:"id"`
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
	// This field is only present on base images, and contains e.g. "claude-code".
	Flavor    string    `json:"flavor,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	Size      int64     `json:"size"`
}

// https://docs.docker.com/reference/cli/sbx/template/save/
func SaveTemplate(
	ctx context.Context,
	sandboxName string,
	templateName string,
) error {
	return sbx(ctx, "template", "save", sandboxName, templateName)
}

// https://docs.docker.com/reference/cli/sbx/template/rm/
func RemoveTemplate(ctx context.Context, templateName string) (stdout []byte, err error) {
	return sbxOutput(ctx, "template", "rm", "--force", templateName)
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

	// Annoyingly, sbx may show an update notice on sbx commands, even when we pass `--json`, which
	// causes `json.Unmarshal` to fail. So we check here if there's an update notice from sbx before
	// unmarshaling JSON, by looking for the strange character "╭" in the output, since the update
	// notice looks like this:
	// ╭──────────────────────────────────────────────────────────────────────────────────╮
	// │ Docker Sandboxes Update Available                                                │
	// ├──────────────────────────────────────────────────────────────────────────────────┤
	// │ v0.49.2  →  v0.50.1                                                              │
	// ├──────────────────────────────────────────────────────────────────────────────────┤
	// │ Release notes  https://github.com/docker/sbx-releases/releases/tag/v0.50.1       │
	// ├──────────────────────────────────────────────────────────────────────────────────┤
	// │ To upgrade     brew upgrade docker/tap/sbx                                       │
	// ╰──────────────────────────────────────────────────────────────────────────────────╯
	indexOfUpdateNotice := bytes.Index(stdout, []byte("╭"))
	if indexOfUpdateNotice != -1 {
		stdout = stdout[0:indexOfUpdateNotice]
	}

	return stdout, nil
}

func sbxJSONOutput[Output any](ctx context.Context, args ...string) (Output, error) {
	var output Output

	stdout, err := sbxOutput(ctx, args...)
	if err != nil {
		return output, err
	}

	if err := json.Unmarshal(stdout, &output); err != nil {
		return output, wrap.ErrorWithAttrs(
			err,
			"failed to parse sbx output as JSON",
			"command", "sbx "+strings.Join(args, " "),
			"output", string(stdout),
		)
	}

	return output, nil
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
