//go:build unix && !linux

package downloader

import "syscall"

func applyDeathSignal(attr *syscall.SysProcAttr) {}
