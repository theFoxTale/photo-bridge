package backend

import (
	"os/exec"
	"syscall"
)

// hideFFmpegWindow stops ffmpeg from opening a console over the Wails window.
func hideFFmpegWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
}
