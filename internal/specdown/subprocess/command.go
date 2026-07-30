package subprocess

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"time"
)

const waitDelay = 500 * time.Millisecond

// CommandContext creates a command whose cancellation terminates the whole
// process tree where the platform supports it. WaitDelay bounds cleanup when a
// descendant keeps inherited pipes open after the direct child exits.
func CommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	configureProcessTree(cmd)
	cmd.Cancel = func() error {
		return killProcessTree(cmd)
	}
	cmd.WaitDelay = waitDelay
	return cmd
}

// Terminate stops cmd and its descendants where the platform supports it.
// An already-exited process is treated as successfully terminated.
func Terminate(cmd *exec.Cmd) error {
	err := killProcessTree(cmd)
	if errors.Is(err, os.ErrProcessDone) {
		return nil
	}
	return err
}
