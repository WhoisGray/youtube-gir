//go:build linux

package downloader

import "syscall"

func applyDeathSignal(attr *syscall.SysProcAttr) {
	// If the server process dies hard, the kernel kills yt-dlp with it.
	// The worker thread that started the process stays locked until Wait.
	attr.Pdeathsig = syscall.SIGKILL
}
