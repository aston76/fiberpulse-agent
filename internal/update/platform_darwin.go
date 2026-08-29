//go:build darwin

package update

import "os/exec"

func platformVerify(path string) error {
	return platformVerificationOutput(exec.Command("/usr/bin/codesign", "--verify", "--strict", path))
}
