# Pi v2

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
as implemented in [pi_solver.go](pi_solver.go).

### pi client [opts] gRPCEndpoint:port

Implements a simple gRPC client that will attempt to make multiple requests
to pi server instances available at the provided endpoint, and
concatenate the responses into a an output.

E.g.

```bash
$ pi client --count 100 endpoint:9090
Result is: 3.1415926535897932384626433832795028841971693993751058209749445923078164062862089986280348253421170679
```

## Installing

```go install github.com/memes/pi/v2```

Pre-built Docker images can be pulled directly from the public [repo](https://hub.docker.com/r/memes/pi/).
