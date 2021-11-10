# Changelog

<!-- markdownlint-disable MD024 -->

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [2.0.0-rc2] - 2021-11-09

### Added

### Changed

- lower prime testing to first 100 when `-short` flag is set
- remove `nobody` from scratch docker image

### Removed

## [2.0.0-rc1] - 2021-11-09

Refactored Pi code as v2 to support use as a library and application. When used
as a server the primary transport is through gRPC, with an optional REST gateway
for compatibility. The client app is always gRPC.

### Added

- protobuf definition for data transfer with [buf](https://buf.build) tooling for
  code generation
- Cache interface definition, with optional Redis implementation for sample server
- goreleaser for binary and container building

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

[2.0.0-rc2]: https://github.com/memes/pi/compare/2.0.0-rc1...2.0.0-rc2
[2.0.0-rc1]: https://github.com/memes/pi/compare/1.0.4...2.0.0-rc1
[1.0.4]: https://github.com/memes/pi/compare/1.0.3...1.0.4
[1.0.3]: https://github.com/memes/pi/compare/1.0.2...1.0.3
[1.0.2]: https://github.com/memes/pi/compare/1.0.1...1.0.2
[1.0.1]: https://github.com/memes/pi/releases/tag/1.0.1
