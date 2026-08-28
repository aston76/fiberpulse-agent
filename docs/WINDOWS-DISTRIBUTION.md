# Windows distribution

The MVP is installed per user in `%LocalAppData%\Programs\FiberPulse`. It does
not require administrator privileges and creates one Task Scheduler logon task.

## Development build

Build the Windows artifacts from the Dev SSD project root:

```sh
docker build -f packaging/windows/Dockerfile.build -t fiberpulse-agent-build .
docker run --rm -v "$PWD/dist:/artifacts" fiberpulse-agent-build
```

The resulting binaries are development artifacts. They are not approved for
public distribution. A release workflow must apply Authenticode SHA-256 with an
RFC 3161 timestamp to the agent, updater and final installer.

## Shutdown contract

`fiberpulse.exe --quit` signals the running per-user instance and waits up to ten
seconds for its singleton mutex to be released. The normal Quit action and the
installer use the same root-context cancellation path: NDT7 is cancelled,
SQLite work is flushed, the loopback server is stopped and the tray is removed.

## Installer gate

The Inno Setup source is ready for CI validation, but a commercial Inno Setup
licence and a recognised Windows signing path remain release blockers. The
uninstaller asks separately before deleting local data; keeping data is the
default.
