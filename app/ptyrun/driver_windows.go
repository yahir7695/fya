//go:build windows

package ptyrun

import (
	"os"
	"os/exec"
)

func startPTY(_ *exec.Cmd, _, _ uint16) (*os.File, error) {
	return nil, ErrUnsupported
}

func killProcessGroup(process *os.Process) error {
	if process == nil {
		return nil
	}
	return process.Kill()
}

func terminateProcessGroup(process *os.Process) error {
	return killProcessGroup(process)
}
