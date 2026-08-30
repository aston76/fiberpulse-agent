# Windows distribution

FiberPulse is installed per user in `%LocalAppData%\Programs\FiberPulse`. It
does not require administrator privileges. The optional start-at-login setting
uses the current user's standard `Run` registry entry and is removed by the
uninstaller.

## Development build

Build the Windows artifacts from the Dev SSD project root:

```sh
docker build -f packaging/windows/Dockerfile.build -t fiberpulse-agent-build .
docker run --rm -v "$PWD/dist:/artifacts" fiberpulse-agent-build
```

The resulting binaries are development artifacts. They are not approved for
public distribution. A release workflow must apply Authenticode SHA-256 with an
RFC 3161 timestamp to the agent, updater and final installer.

## Microsoft Store package

The Store edition is a separate MSIX package. It uses the identity assigned by
Partner Center (`SEOWEBAPP.FiberPulse`) and is signed and updated by Microsoft
after certification. It intentionally does not contain the standalone updater.

CI builds and structurally verifies the development Store package on Windows.
The exact artifact to upload in Partner Center is named
`FiberPulse-0.1.0.0-windows-x64.msix`. The package must remain unsigned before
Store upload; Microsoft applies the trusted Store signature during publishing.

## Shutdown contract

`fiberpulse.exe --quit` signals the running per-user instance and waits up to ten
seconds for its singleton mutex to be released. The normal Quit action and the
installer use the same root-context cancellation path: NDT7 is cancelled,
SQLite work is flushed, the loopback server is stopped and the tray is removed.

## Installer gate

CI compiles and silently installs the Inno Setup package to verify the exact
files shipped to users. Public release remains blocked unless the agent,
updater and final installer all carry a valid Authenticode signature and RFC
3161 timestamp. The uninstaller asks separately before deleting local data;
keeping data is the default.
