# macOS compatibility

FiberPulse supports macOS 13 or later on Apple Silicon and Intel. The Go core,
SQLite history, loopback dashboard, scheduler, reports and full Quit path are
shared with Windows. A native menu-bar item exposes Open, Test, Pause, Report,
Update and Quit actions without WebView, Node or Python at runtime.

## Development bundle

On a macOS build host with Go 1.26.7:

```sh
chmod +x packaging/macos/build-app.sh
packaging/macos/build-app.sh
```

The script cross-builds both `arm64` and `x86_64`, combines them with `lipo`,
then creates an ad-hoc-signed universal `.app` and ZIP for internal testing.
The release workflow can switch to hardened-runtime Developer ID signing.
Public distribution remains blocked until the resulting archive is notarized,
stapled and accepted by Gatekeeper. An ad-hoc signature is never presented as
a public release signature.

The one-line installer installs the bundled LaunchAgent for start-at-login. It
restarts only after a crash; a normal Quit is not restarted. The installer
replaces the executable and log placeholders with absolute per-user paths.
