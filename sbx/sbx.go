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
