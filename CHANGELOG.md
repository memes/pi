# Changelog

<!-- spell-checker: ignore markdownlint -->
<!-- markdownlint-disable MD024 -->

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [2.0.0-rc5] - 2021-08-24

### Added

### Changed

- release action: rename to `release`
- metadata labels is now a map of strings
- updated buf repository for googleapis; regenerated files from protobuf

### Removed

## [2.0.0-rc4] - 2021-08-21

### Added

### Changed

- go-release action: login to Docker hub before attempting to publish image

### Removed

## [2.0.0-rc3] - 2021-08-21

Switch to [goreleaser](https://goreleaser.com/intro/) for building cross-platform
binaries and container image.

### Added

- `goreleaser` configuration for binaries and publishing

### Changed

- go-release action to use `goreleaser`
- `server`: default to not starting REST gateway
- `server`: add option to enable REST gateway
- `server`: corrected gRPC health check path

### Removed

## [2.0.0-rc2] - 2021-07-12

### Added

### Changed

- Response metadata as a distinct and extensible protobuf type

### Removed

## [2.0.0-rc1] - 2021-07-11

First test of refactored Pi code as v2; gRPC is primary transport with a REST
gateway for compatibility.

### Added

- protobuf definition for data transfer with [buf](https://buf.build) tooling for
  code generation
- Cache interface definition, with Redis implementation for sample server

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

[2.0.0-rc5]: https://github.com/memes/pi/compare/2.0.0-rc4...2.0.0-rc5
[2.0.0-rc4]: https://github.com/memes/pi/compare/2.0.0-rc3...2.0.0-rc4
[2.0.0-rc3]: https://github.com/memes/pi/compare/2.0.0-rc2...2.0.0-rc3
[2.0.0-rc2]: https://github.com/memes/pi/compare/2.0.0-rc1...2.0.0-rc2
[2.0.0-rc1]: https://github.com/memes/pi/compare/1.0.4...2.0.0-rc1
[1.0.4]: https://github.com/memes/pi/compare/1.0.3...1.0.4
[1.0.3]: https://github.com/memes/pi/compare/1.0.2...1.0.3
[1.0.2]: https://github.com/memes/pi/compare/1.0.1...1.0.2
[1.0.1]: https://github.com/memes/pi/releases/tag/1.0.1
