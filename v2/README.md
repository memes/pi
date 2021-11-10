# Pi v2

A simple distributed computing example that calculates the fractional decimal digit of
pi at the requested index.

Using a [Bailey-Borwein-Plouffe](https://en.wikipedia.org/wiki/Bailey%E2%80%93Borwein%E2%80%93Plouffe_formula) spigot algorithm, any 9 consecutive digits of pi can be calculated without knowledge of the digits that preceded them. This allows the creation of a solution for a sequence of pi digits that is [embarrassingly parallel](https://en.wikipedia.org/wiki/Embarrassingly_parallel).

This library and sample application demonstrates one way the problem of calculating the first N fractional decimal digits of pi could be distributed amongst workers calculating sections of pi independently, and in parallel.

I.e. the first 18 fractional decimal digits of pi are `141592653589793238`.
Executing `BBPDigits(n)` function in [pi.go](pi.go) for `n = [0...9]` will return
these results.

| index | return |
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

From this, we can see that generating the first 18 fractional digits of pi can be achieved by concatenating the results of `BBPDigits(0)` and `BBPDigits(9)`, and can be extended to any arbitrary number of fractional digits by requesting the next 9 digits, and so on.

Pseudo-code:

```pseudo
digits = ""
for i = 0; i < N; i += 9 {
    digits += BBPDigits(i)
}
print digits
```

## Sample Application

> NOTE: The intent of this application is to demonstrate distributed computing and scaling. There are better ways to calculate pi to arbitrary precision if speed or accuracy is required!

The bundled application uses the approach to assemble the digits of pi through multiple requests to services that return a single fractional digit of pi. This generates a fair amount of network traffic and is sufficient to demonstrate various autoscaling options in public cloud and Kubernetes scenarios.

### pi client

Implements a simple gRPC client that will attempt to make multiple requests
to pi server instances available at the provided endpoints, and concatenate the
responses into a an output.

E.g. to request the first 250 fractional digits of pi from a gRPC server
running at localhost:9090:

```shell
pi client --count 250 --timeout 25s localhost:9090
```

```text
Result is: 3.1415926535897932384626433832795028841971693993751058209749445923078164062862089986280348253421170679821480865132823066470938446095505822317253594081284811174502841027019385211055596446229489549303819644288109756659334461284756482337867831652712019091
```

Use `pi help client` to see the full set of options available to the client.

### pi server

Launches a service that listens for incoming gRPC (and REST) requests and returns
the matching digit, optionally from a Redis cache. If there is a cache-miss, the
service calculates the digits using the library functions in `v2`.

E.g. to launch the server with verbose logging

```shell
pi server --verbose
```

Use `pi help server` to see the full set of options available to the client.

#### REST gateway

The `pi server` can optionally enable an embedded REST-to-gRPC gateway to expose a REST API for the server.

E.g. launch server with REST gateway (default listener `:8080`)

```shell
pi server --otlp-endpoint otlp-collector:4317 --enable-rest --label foo=bar
```

Using `httpie` to test the endpoint

```shell
http :8080/api/v2/digit/0
```

```text
HTTP/1.1 200 OK
Content-Length: 225
Content-Type: application/json
Date: Wed, 10 Nov 2021 04:13:21 GMT
Grpc-Metadata-Content-Type: application/grpc
Traceparent: 00-fb4d915bde98587a1ccb8e813fedaaec-7e3739106904a4a0-01

{
    "digit": 1,
    "index": "0",
    "metadata": {
        "addresses": [
            "2001:xxx:xxxx:0:1cab:3ec7:9479:7b0f",
            "2001:xxx:xxxx:0:553b:f6f0:bf6e:bce7",
            "172.16.xxx.xxx",
            "2001:xxx:xxxx::ffff:e7bd"
        ],
        "identity": "server.example.com",
        "labels": {
            "foo": "bar"
        }
    }
}
```

### OpenTelemetry metrics and traces

Both `pi server` and `pi client` support an optional flag to set an OpenTelemetry collector where span and metric data will be sent. Use the `--otlp-endpoint` flag with an address:port specifier, and configure the collector to relay traces and metrics to any supported analyser.

## Installing binaries

The command line client-server application can be installed directly from the v2
repository.

```shell
go install github.com/memes/pi/v2/cmd/pi
```

The `pi` binary will be added to your `GOBIN` directory.

Tagged binaries are also published and can be downloaded from [GitHub Releases](https://github.com/memes/pi/releases).

## Docker containers

In addition, tagged releases are published to the public
[Docker hub repo](https://hub.docker.com/r/memes/pi/). The container defaults to
running `pi server` without any options. To change the action and add options just
add the arguments to `docker run` or `podman run`.

E.g. to run the latest v2 `pi server` with metrics and traces published to an
OpenTelemetry collector at `otlp-collector:4317`:-

```shell
docker run --rm memes/pi:2 server --otlp-endpoint otlp-collector:4317
```
