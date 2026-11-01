//go:build !unix

package downloader

import "os/exec"

func configureCommand(cmd *exec.Cmd) {}

func signalProcess(cmd *exec.Cmd, force bool) {
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
