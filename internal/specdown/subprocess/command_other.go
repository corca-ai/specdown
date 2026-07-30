//go:build !darwin && !linux && !windows

package subprocess

import (
	"os"
	"os/exec"
)

func configureProcessTree(_ *exec.Cmd) {}

func killProcessTree(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return os.ErrProcessDone
	}
	return cmd.Process.Kill()
}
