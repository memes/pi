# Changelog

<!-- markdownlint-disable MD024 -->

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [2.0.3](https://github.com/memes/pi/compare/v2.0.2...v2.0.3) (2023-11-13)


### Bug Fixes

* gRPC 1.58.0 adds error to xds.NewGRPCServer ([541d07a](https://github.com/memes/pi/commit/541d07a3e9e8e9bb9a32f1c0d9200fcc4103203e))
* Make Go 1.21 the base version ([8445a7b](https://github.com/memes/pi/commit/8445a7b0a2c2a62606d0b51ef6a5327d01ecb239))
* Update OTEL dependencies to 0.16 ([799dd30](https://github.com/memes/pi/commit/799dd303f23f0bcc5a6c8d9e951cf969fad417c8))
* Update OTEL libraries ([d8960b9](https://github.com/memes/pi/commit/d8960b95eee29d6ec89578e4b9a30a4eb98c3fff))
* Update OTEL packages to 1.15.1/0.38.1 ([cc8b4d5](https://github.com/memes/pi/commit/cc8b4d55154887bf32ef3e2d448e23681b51b58b))

## [2.0.2](https://github.com/memes/pi/compare/v2.0.1...v2.0.2) (2023-03-25)


### Bug Fixes

* Execute goreleaser action in v2 folder ([e65fe4c](https://github.com/memes/pi/commit/e65fe4cc2c9639f66d96257c817ec9b3a51e6e13))

## [2.0.1](https://github.com/memes/pi/compare/v2.0.0...v2.0.1) (2023-03-25)


### Bug Fixes

* Bump to go 1.20 ([6d7dd73](https://github.com/memes/pi/commit/6d7dd73976d25d53156057385ab35770ce2f1c6c))
* Lint fixes ([192a782](https://github.com/memes/pi/commit/192a782579b6617cab9e15023e6ab30766bfafe3))
* Remove pi v1 code ([2b2d51c](https://github.com/memes/pi/commit/2b2d51c992d86b1761f71477657a9f036061689d))
* Resolve failing example ([fa007b7](https://github.com/memes/pi/commit/fa007b7c8451a64828db43ed39367264f5526b89))
* Update pi v2 for otel v1.14.0/v0.37.0 ([5afba4a](https://github.com/memes/pi/commit/5afba4a58adeafdfd1edddc10c1f2f1af62c522c))

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
