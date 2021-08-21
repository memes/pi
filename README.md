# Pi

A simple distributed computing example that calculates the mantissa of
pi at the requested index. Using a spigot algorithm, digits of the
mantissa can be calculated independently, and in parallel.

I.e. the first 6 digits of pi are 3.14159; if a request is made for index 0, the service will return 1.

| index | return |
|-------|--------|
| 0 | 1 |
| 1 | 4 |
| 2 | 1 |
| 3 | 5 |
| 4 | 9 |

## Usage

### pi server [opts]

Launches a service that listens for incoming gRPC (and REST) requests and returns
the matching digit, optionally from a Redis cache. If there is a cache-miss, the
service calculates the digits using a [Bailey-Borwein-Ploufee
algorithm](https://en.wikipedia.org/wiki/Bailey%E2%80%93Borwein%E2%80%93Plouffe_formula)
as implemented in [pidigits.go](pkg/pidigits.go).

### pi client [opts] gRPCEndpoint:port

Implements a simple gRPC client that will attempt to make multiple requests
to pi server instances available at the provided endpoint, and
concatenate the responses into a an output.

E.g.

```bash
$ pi client --count 100 endpoint:9090
Result is: 3.1415926535897932384626433832795028841971693993751058209749445923078164062862089986280348253421170679
```

## Building

This is a fully contained Go application; after checkout a simple
```go install github.com/memes/pi``` will build the apps;
[dep](https://github.com/golang/dep) is used for vendoring.

The application can be deployed to Docker and pre-built images can be
pulled directly from the public
[repo](https://hub.docker.com/r/memes/pi/).
