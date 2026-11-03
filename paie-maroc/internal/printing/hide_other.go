//go:build !windows

package printing

import "os/exec"

// hideConsole only matters on Windows, where PowerShell would otherwise
// flash a console window.
func hideConsole(*exec.Cmd) {}
