//go:build darwin

package update

import (
	"os"
	"os/exec"
)

func platformVerify(path string) error {
	arguments := []string{"--verify", "--strict"}
	isBundle := false
	if info, err := os.Lstat(path); err == nil && info.IsDir() {
		isBundle = true
		arguments = append(arguments, "--deep")
	}
	arguments = append(arguments, path)
	if err := platformVerificationOutput(exec.Command("/usr/bin/codesign", arguments...)); err != nil {
		return err
	}
	if isBundle {
		return platformVerificationOutput(exec.Command("/usr/sbin/spctl", "--assess", "--type", "execute", path))
	}
	return nil
}
