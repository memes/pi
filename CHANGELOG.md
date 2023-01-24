# Changelog

<!-- markdownlint-disable MD024 -->

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [v2.0.0] - 2023-01-23

> NOTE: Entries for -rc1 through -rc4 have been removed as the tags and builds
> for those have been removed. These notes include all changes from the `1.0.4`
> tag to `v2.0.0`.

Refactored Pi code as [v2](/v2) to support use as a library and application.
When used as a server the primary transport is through gRPC, with an optional
REST gateway for compatibility. The client app always uses gRPC transport.

### Added

- gRPC is primary transport for client and server, with optional REST-gRPC gateway
  support
- switch to [buf](https://buf.build) tooling for code generation from protobuf
- OpenTelemetry tracing and metric collector support in application
- Use goreleaser for binary and container building via GitHub action on tag
  - SBOM generation through [syft](https://github.com/anchore/syft)
  - [cosign](https://github.com/sigstore/cosign) keyless signed containers
  on tag
- please-release GitHub action to drive release process

### Changed

- Separated pi digit library implementation from command line applications

### Removed

## [1.0.4] - 2021-06-15

### Added

### Changed

- GitHub release action; build `pi` for Windows

### Removed

## [1.0.3] - 2021-06-15

### Added

- Dockerfile

### Changed

- Transitioned to GO modules

### Removed

<!-- spell-checker: ignore vendored -->
- Vendored dependencies

## [1.0.2] - 2021-06-15

### Added

- CHANGELOG and CONTRIBUTING docs
- pre-commit and GitHub actions

### Changed

### Removed

## [1.0.1] - 2017-09-26

### Added

- Tagged to match the Docker hub image published in 2017.

### Changed

### Removed

[v2.0.0]: https://github.com/memes/pi/compare/1.0.4...v2.0.0
[1.0.4]: https://github.com/memes/pi/compare/1.0.3...1.0.4
[1.0.3]: https://github.com/memes/pi/compare/1.0.2...1.0.3
[1.0.2]: https://github.com/memes/pi/compare/1.0.1...1.0.2
[1.0.1]: https://github.com/memes/pi/releases/tag/1.0.1
