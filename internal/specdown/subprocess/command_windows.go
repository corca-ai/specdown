//go:build windows

package subprocess

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strconv"
)

func configureProcessTree(_ *exec.Cmd) {}

func killProcessTree(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return os.ErrProcessDone
	}
	ctx, cancel := context.WithTimeout(context.Background(), waitDelay)
	defer cancel()
	treeErr := exec.CommandContext(ctx, "taskkill", "/PID", strconv.Itoa(cmd.Process.Pid), "/T", "/F").Run()
	if treeErr == nil {
		return nil
	}
	killErr := cmd.Process.Kill()
	if killErr == nil {
		return treeErr
	}
	if errors.Is(killErr, os.ErrProcessDone) {
		return treeErr
	}
	return errors.Join(treeErr, killErr)
}
