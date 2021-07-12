FROM golang:1.16.5-alpine AS builder
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o pi ./cmd

FROM alpine:3.13.5
ARG COMMIT_SHA="main"
ARG TAG_NAME="unreleased"
LABEL maintainer="Matthew Emes <memes@matthewemes.com>" \
      org.opencontainers.image.title="pi" \
      org.opencontainers.image.authors="memes@matthewemes.com" \
      org.opencontainers.image.description="Calculate digits of pi" \
      org.opencontainers.image.url="https://github.com/memes/pi" \
      org.opencontainers.image.source="https://github.com/memes/pi/tree/${COMMIT_SHA}" \
      org.opencontainers.image.documentation="https://github.com/memes/pi/tree/${COMMIT_SHA}/README.md" \
      org.opencontainers.image.version="${TAG_NAME}" \
      org.opencontainers.image.revision="${COMMIT_SHA}" \
      org.opencontainers.image.licenses="MIT" \
      org.label-schema.schema-version="1.0" \
      org.label-schema.name="pi" \
      org.label-schema.description="Calculate digits of pi" \
      org.label-schema.url="https://github.com/memes/pi" \
      org.label-schema.vcs-url="https://github.com/memes/pi/tree/${COMMIT_SHA}" \
      org.label-schema.usage="https://github.com/memes/pi/tree/${COMMIT_SHA}/README.md" \
      org.label-schema.version="${TAG_NAME}" \
      org.label-schema.vcs-ref="${COMMIT_SHA}" \
      org.label-schema.license="MIT"

RUN apk --no-cache add ca-certificates=20191127-r5
WORKDIR /run
COPY --from=builder /src/pi /usr/local/bin/
EXPOSE 8080
EXPOSE 9090
USER nobody
ENTRYPOINT ["/usr/local/bin/pi"]
CMD ["server"]
