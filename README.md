# FiberPulse Agent

Local-first Windows 10/11 and macOS 13+ Internet-performance agent. The agent does not run M-Lab
or share FiberPulse data until the corresponding, separate consent is recorded.

## Development

The supported toolchain is Go 1.26.7 and Node 24 LTS. The repository includes
containerized build targets so the host does not need either runtime.

```sh
make test
make dashboard
make build
make windows
make macos
```

Native Windows artifacts are produced with the repository's MinGW cross-build.
Universal macOS `.app` bundles containing both Apple Silicon and Intel slices
are built on macOS with `make macos`; GitHub CI verifies both macOS 14 and 15.
Neither development artifact is approved for
public distribution until its platform signing gate is complete.

## Command-line installation

After the first signed public release is available, install FiberPulse with one
command. The bootstrap scripts refuse development artifacts and verify the
native platform signature before installing anything.

macOS Terminal:

```sh
curl -fsSL https://raw.githubusercontent.com/aston76/fiberpulse-agent/main/install.sh | sh
```

Windows PowerShell:

```powershell
irm https://raw.githubusercontent.com/aston76/fiberpulse-agent/main/install.ps1 | iex
```

Until the signing and notarization gates are complete, both commands stop with
a clear message and make no installation change.

The updater helper validates a signed, expiring manifest, semantic version and
monotonic sequence, artifact hash and size, and the native platform signature.
It starts the replacement agent, waits for a PID-bound health receipt, and
restores and restarts the prior binary if startup fails. See
[`docs/UPDATE-SECURITY.md`](docs/UPDATE-SECURITY.md) for the manifest contract
and the remaining release gates.

Real M-Lab NDT7 measurement is the default provider. It runs only after M-Lab
consent is recorded in the local dashboard and stays within the scheduled
quota of four automatic tests per day. Set `FIBERPULSE_DEV_FAKE=1` to force
the deterministic fake provider during development; automated Go tests always
use the fake provider.

## License

Apache-2.0. The FiberPulse name and third-party services are not licensed by
this repository.
