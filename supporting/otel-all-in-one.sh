#!/bin/sh
#
# Quick script to run OpenTelemetry collector with Jaeger and Prometheus, in
# podman, with optional TLS support.

LIBDIR="$(readlink -f `dirname $0`)"
if [ -e "${LIBDIR}/otel-all-in-one-tls.yaml" ]; then
    CONFIG="${LIBDIR}/otel-all-in-one-tls.yaml"
else
    CONFIG="${LIBDIR}/otel-all-in-one.yaml"
fi

# Shutdown existing deployment, if running, and any existing ConfigMap volumes
test -z "$(podman pod ps --filter name=$(basename "${CONFIG}" .yaml) --format '{{ .Id }}')" || \
    podman play kube --down ${CONFIG}
podman volume ls --format '{{ .Name }}' | \
    grep -E '^otel-(?:[^-]+-config|collector-tls-[0-9a-z]+)$' | \
    xargs -J % podman volume rm % --force

# Launch OpenTelemetry, Prometheus and Jaeger
podman play kube ${CONFIG}
