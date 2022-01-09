package util

import (
	"os/exec"
)

// Not using "command -v" because it doesn't work with Windows.
// testArg will usually be something like --version.
func CheckInstalled(program, testArg string) bool {
	cmd := exec.Command(program, testArg)
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}
