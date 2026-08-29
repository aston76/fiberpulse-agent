//go:build darwin || windows

package update

import (
	"errors"
	"os/exec"
	"strings"
)

func platformVerificationOutput(command *exec.Cmd) error {
	output, err := command.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return errors.New(message)
	}
	return nil
}
