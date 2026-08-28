# macOS compatibility

FiberPulse supports macOS 13 or later on Apple Silicon and Intel. The Go core,
SQLite history, loopback dashboard, scheduler, reports and full Quit path are
shared with Windows. A native menu-bar item exposes Open, Test, Pause, Report,
Update and Quit actions without WebView, Node or Python at runtime.

## Development bundle

On a macOS build host with Go 1.26:

```sh
chmod +x packaging/macos/build-app.sh
packaging/macos/build-app.sh
```

The script cross-builds both `arm64` and `x86_64`, combines them with `lipo`,
then creates an ad-hoc-signed universal `.app` and ZIP for internal testing.
Public distribution remains blocked until a Developer ID Application certificate,
hardened-runtime signing, notarization and stapling are configured. An ad-hoc
signature is never presented as a public release signature.

The optional LaunchAgent template restarts only after a crash. A normal Quit is
not restarted. Its executable and log placeholders must be replaced by the
installer with absolute per-user paths.
