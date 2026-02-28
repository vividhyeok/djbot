//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

// hideWindow prevents the command from flashing a console window on Windows.
func hideWindow(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
}
