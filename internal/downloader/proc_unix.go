//go:build unix

package downloader

import (
	"os/exec"
	"syscall"
)

func configureCommand(cmd *exec.Cmd) {
	attr := &syscall.SysProcAttr{Setpgid: true}
	applyDeathSignal(attr)
	cmd.SysProcAttr = attr
}

func signalProcess(cmd *exec.Cmd, force bool) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	sig := syscall.SIGINT
	if force {
		sig = syscall.SIGKILL
	}
	// Negative pid signals the process group, including ffmpeg children.
	_ = syscall.Kill(-cmd.Process.Pid, sig)
}
