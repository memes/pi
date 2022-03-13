#!/bin/sh
#
# Quick script to run OpenTelemetry collector with Jaeger and Prometheus, in
# podman with optional TLS support.

# Shutdown existing deployment, if running
test -z "$(podman pod ps --filter name=otel-all-in-one --format '{{ .Id }}')" || \
    podman play kube --down otel-all-in-one.yaml

# Create a volume for Prometheus and OpenTelemetry config files; podman play kube
# doesn't support ConfigMap volume mounts, but does support using PVCs with volumes
podman volume exists otel-all-in-one-prom-config || \
    podman volume create otel-all-in-one-prom-config
podman volume exists otel-all-in-one-otel-config || \
    podman volume create otel-all-in-one-otel-config
_container=$(podman create \
    --mount type=volume,src=otel-all-in-one-prom-config,target=/mnt/prom-config \
    --mount type=volume,src=otel-all-in-one-otel-config,target=/mnt/otel-config \
    otel/opentelemetry-collector:0.46.0)
tar cf - prometheus.yml | podman cp - "${_container}:/mnt/prom-config/"
tar cf - otel-collector-config.yaml otel.pi.example.com.pem otel.pi.example.com-key.pem | \
    podman cp - "${_container}:/mnt/otel-config"
podman rm "${_container}"

# Launch OpenTelemetry, Prometheus and Jaeger
podman play kube otel-all-in-one.yaml
