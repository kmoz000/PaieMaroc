//go:build windows

package printing

import (
	"os/exec"
	"syscall"
)

// createNoWindow is CREATE_NO_WINDOW, absent from the stdlib syscall package.
const createNoWindow = 0x08000000

// hideConsole prevents the black console window that would otherwise flash
// each time the GUI application shells out to PowerShell.
func hideConsole(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
}
