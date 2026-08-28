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
```

Native Windows artifacts are produced with the repository's MinGW cross-build.
Universal macOS `.app` bundles containing both Apple Silicon and Intel slices
are built on macOS with `make macos`; GitHub CI verifies both macOS 14 and 15.
Neither development artifact is approved for
public distribution until its platform signing gate is complete.

Real M-Lab execution is disabled in ordinary development builds. It additionally
requires recorded M-Lab consent and `FIBERPULSE_ENABLE_MLAB_DEV=1`. Automated
tests use the deterministic fake provider.

## License

Apache-2.0. The FiberPulse name and third-party services are not licensed by
this repository.
