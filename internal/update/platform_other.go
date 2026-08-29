//go:build !darwin && !windows

package update

import "errors"

func platformVerify(string) error {
	return errors.New("platform signature verification is unavailable on this operating system")
}
