# Update security contract

`fiberpulse-updater` is a low-level replacement helper. It is not an update
discovery service and it does not download artifacts. The main application must
download a candidate to a staging path and invoke the helper only after the
user-approved update flow has selected a release.

## Required inputs

All file paths are absolute. The target, staged artifact, manifest and existing
anti-rollback state must be regular files rather than symbolic links. The
helper receives the installed semantic version, expected `stable` or `canary`
channel, Ed25519 public key and health timeout explicitly.

The signed JSON manifest contains exactly these fields:

```json
{
  "version": "1.2.3",
  "channel": "stable",
  "sequence": 42,
  "sha256": "64-lowercase-hex-characters",
  "size": 12345678,
  "url": "https://updates.example/fiberpulse-1.2.3.exe",
  "minimum_version": "1.0.0",
  "expires_at": "2026-09-01T00:00:00Z",
  "signature": "base64-ed25519-signature"
}
```

To sign, omit `signature`, encode the remaining Go manifest structure as compact
JSON in the field order above, and sign those exact bytes with the offline
Ed25519 release key. Unknown fields and trailing JSON are rejected. The URL must
be absolute HTTPS without embedded credentials. The sequence must be strictly
greater than the last successfully installed sequence, the version must be
newer than the installed version, and the manifest must not be expired.

## Replacement and rollback

Before replacement, the staged file must match the declared SHA-256 and byte
size and pass native signature verification: Authenticode status `Valid` on
Windows, or strict code-signing verification on macOS. The helper retains one
`*.previous` binary and refuses to overwrite ambiguous recovery data.

After replacement, the helper launches the agent with a reserved health-receipt
path. Success requires a receipt containing both the expected version and the
actual child PID while that process remains alive. Failure or timeout kills the
candidate, restores the previous binary and restarts it. Anti-rollback state is
advanced only after the health check succeeds.

## Distribution gates

Development builds are signed ad hoc on macOS and are not public release
artifacts. Windows release artifacts require Authenticode SHA-256 and RFC 3161
timestamping. A public macOS updater must replace the complete Developer-ID
signed and notarized `.app` bundle; replacing only its inner executable would
invalidate the bundle seal. The current dashboard `Check for update` action is
therefore intentionally non-operational until release signing, download,
notarization and full-bundle replacement are wired and tested.
