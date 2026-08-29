//go:build windows

package update

import "os/exec"

func platformVerify(path string) error {
	const script = `$signature = Get-AuthenticodeSignature -LiteralPath $args[0]; if ($signature.Status -ne 'Valid') { Write-Error ('Authenticode status: ' + $signature.Status); exit 1 }`
	return platformVerificationOutput(exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script, path))
}
