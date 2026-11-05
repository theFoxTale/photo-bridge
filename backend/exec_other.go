//go:build !windows

package backend

import "os/exec"

func hideFFmpegWindow(cmd *exec.Cmd) {}
