# Fractional digits of pi, v2

[![Go Reference](https://pkg.go.dev/badge/github.com/memes/pi/v2.svg)](https://pkg.go.dev/github.com/memes/pi/v2)
[![Go Report Card](https://goreportcard.com/badge/github.com/memes/pi/v2)](https://goreportcard.com/report/github.com/memes/pi/v2)

A simple distributed computing example that calculates the fractional decimal digit of
&#x1D745; at the requested index.

This library and sample application demonstrates one way the problem of calculating
the subset N fractional decimal digits of pi could be distributed amongst workers
calculating sections of pi independently, and in parallel.

> NOTE: Development of v1 package `github.com/memes/pi` has been suspended and all
> updates will be to [github.com/memes/pi/v2](v2/) only.

Contributions are welcome! Please review [Contributions](CONTRIBUTING.md) for
more information.

## Sample Application

> NOTE: The intent of this application is to demonstrate distributed computing
> and scaling. There are better ways to calculate pi to arbitrary precision if
> speed or accuracy is required!

The bundled application assembles the digits of pi through multiple requests to
services that return a single fractional digit. This generates a fair amount of
network traffic and is sufficient to demonstrate various autoscaling options in
public cloud and Kubernetes scenarios.

See [Background](#background) below for more details on the algorithm used to
calculate the digits of pi.

### pi client

Implements a simple gRPC client that will attempt to make multiple requests
to pi server instances available at the provided endpoint, and concatenate the
responses into a an output.

E.g. to request the first 250 fractional digits of pi from an [insecure](#tls-mutual-tls-and-grpc-authorities) gRPC server
running at `localhost:8443`:

```shell
pi client collate --count 250 --max-timeout 25s --insecure localhost:8443
```

```text
Result is: 3.1415926535897932384626433832795028841971693993751058209749445923078164062862089986280348253421170679821480865132823066470938446095505822317253594081284811174502841027019385211055596446229489549303819644288109756659334461284756482337867831652712019091
```

Use `pi help client` to see the full set of options available to the client.

### pi server

Launches a service that listens for incoming gRPC (and REST) requests and returns
the matching digit, optionally from a Redis cache. If there is a cache-miss, the
service calculates the digits using the library function in [pi/v2].

E.g. to launch the server with verbose logging

```shell
pi server --verbose
```

Use `pi help server` to see the full set of options available to the client.

#### Enable the REST gateway

The `pi server` can optionally enable an embedded REST-to-gRPC gateway to expose
a generated REST API for the service. The gateway is enabled by specifying the
`--rest-address` that the HTTP/1 service will bind to.

A generated OpenAPI v2 definition can be retrieved from the REST endpoint
at `/api/v2/swagger.json`.

For example, using a `pi server` with REST gateway on port 8080 and [httpie] as
a client:

```shell
pi server --rest-address :8080 --annotation foo=bar --tag red
```

```shell
http :8080/api/v2/digit/0
```

```text
HTTP/1.1 200 OK
Content-Length: 116
Content-Type: application/json
Date: Tue, 19 Apr 2022 05:00:54 GMT
Grpc-Metadata-Content-Type: application/grpc

{
    "digit": 1,
    "index": "0",
    "metadata": {
        "annotations": {
            "foo": "bar"
        },
        "identity": "server.example.com",
        "tags": [
            "red"
        ]
    }
}
```

### OpenTelemetry metrics and traces

Both `pi server` and `pi client` support an optional flag to set an OpenTelemetry
collector where span and metric data will be sent, annotated with any `tags` or
`annotations` set on the `pi server`. Use the `--otlp-target` flag with an
`[address:]port` specifier to send telemetry to a collector.

E.g.

```shell
pi server --otlp-target collector:4317 --otlp-insecure
```

### TLS, mutual TLS, and gRPC authorities

gRPC connections initiated by a client are expected to be served by a receiver
with a verifiable TLS certificate, which matches the DNS name or address of the
server endpoint. If `pi server` is using a certificate issued by a CA with a trusted
certificate, or is deployed behind a load balancer that is terminating TLS with
a trusted certificate, `pi client` should be able to establish a secure gRPC
connection without further configuration. There are flags that can change the
behavior of the application; see `pi help client` and/or `pi help server` for
all the configuration options.

## Binaries

Binaries are published on the [Releases] page for Linux, macOS, and Windows. If
you have Go installed locally, `go install github.com/memes/pi/v2/cmd/pi@latest`
will download and install to *$GOBIN*.

A container image is also published to Docker Hub and GitHub Container Registries
that can be used in place of the binary; just append the arguments to the
`docker run` or `podman run` command.

E.g. to run the latest v2 `pi server` with verbose logging, and with metrics and
traces published to an OpenTelemetry gRPC collector at `collector:4317`:

```sh
podman run --rm ghcr.io/memes/pi:2 server --verbose --verbose --otlp-target collector:4317
```

## Verifying releases

For each tagged release, an tarball of the source and a [syft] SBOM is created,
along with SHA256 checksums for all files. [cosign] is used to automatically generate
a signing certificate for download and verification of container images.

### Verify release files

1. Download the checksum, signature, and signing certificate file from GitHub

   ```shell
   curl -sLO https://github.com/memes/pi/releases/download/v2.0.0-rc7/pi_2.0.0-rc7_SHA256SUMS
   curl -sLO https://github.com/memes/pi/releases/download/v2.0.0-rc7/pi_2.0.0-rc7_SHA256SUMS.sig
   curl -sLO https://github.com/memes/pi/releases/download/v2.0.0-rc7/pi_2.0.0-rc7_SHA256SUMS.pem
   ```

2. Verify the SHA256SUMS have been signed with [cosign]

   ```shell
   cosign verify-blob --cert pi_2.0.0-rc7_SHA256SUMS.pem --signature pi_2.0.0-rc7_SHA256SUMS.sig pi_2.0.0-rc7_SHA256SUMS
   ```

   ```text
   verified OK
   ```

3. Download and verify files

   Now that the checksum file has been verified, any other file can be verified using `sha256sum`.

   For example

   ```shell
   curl -sLO https://github.com/memes/pi/releases/download/v2.0.0-rc7/pi-2.0.0-rc7.tar.gz.sbom
   curl -sLO https://github.com/memes/pi/releases/download/v2.0.0-rc7/pi_2.0.0-rc7_linux_amd64
   sha256sum --ignore-missing -c pi_2.0.0-rc7_SHA256SUMS
   ```

   ```text
   pi-2.0.0-rc7.tar.gz.sbom: OK
   pi_2.0.0-rc7_linux_amd64: OK
   ```

### Verify container image

Use [cosign]s experimental OCI signature support to validate the container.

```shell
COSIGN_EXPERIMENTAL=1 cosign verify ghcr.io/memes/pi:v2.0.0-rc7
```

## Background

Using a [Bailey-Borwein-Plouffe] spigot algorithm, any 9 consecutive digits of pi
can be calculated without knowledge of the digits that preceded them. This allows
the development of a solution for a sequence of pi digits that appears to be
[embarrassingly parallel].

I.e. the first 18 fractional decimal digits of pi are `141592653` `589793238`.
Executing `BBPDigits(n)` function in [pi.go](pi.go) for `n = [0...9]` will return
these results.

| index | result |
|-------|--------|
| **0** | **141592653** |
| 1 | 415926535 |
| 2 | 159265358 |
| 3 | 592653589 |
| 4 | 926535897 |
| 5 | 265358979 |
| 6 | 653589793 |
| 7 | 535897932 |
| 8 | 358979323 |
| **9** | **589793238** |

From this, we can see that generating the first 18 fractional digits of pi can be
achieved by concatenating the results of `BBPDigits(0)` and `BBPDigits(9)`, and
can be extended to any arbitrary number of fractional digits by requesting the
next 9 digits, and so on.

Pseudo-code:

```pseudo
digits = ""
for i = 0; i < N; i += 9 {
    digits += BBPDigits(i)
}
print digits
```

[Bailey-Borwein-Plouffe]: https://en.wikipedia.org/wiki/Bailey%E2%80%93Borwein%E2%80%93Plouffe_formula
[embarrassingly parallel]: https://en.wikipedia.org/wiki/Embarrassingly_parallel
[httpie]: https://github.com/httpie/httpie
[Releases]: https://github.com/memes/pi/releases
[cosign]: https://github.com/SigStore/cosign
[syft]: https://github.com/anchore/syft
