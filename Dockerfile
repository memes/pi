FROM alpine:3.13.5 as ca
RUN apk --no-cache add ca-certificates-bundle=20191127-r5

FROM scratch
COPY --from=ca /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
EXPOSE 8080
EXPOSE 9090
LABEL maintainer="Matthew Emes <memes@matthewemes.com>" \
      org.opencontainers.image.title="pi" \
      org.opencontainers.image.authors="memes@matthewemes.com" \
      org.opencontainers.image.description="Calculate digits of pi" \
      org.opencontainers.image.url="https://github.com/memes/pi" \
      org.opencontainers.image.licenses="MIT" \
      org.label-schema.schema-version="1.0" \
      org.label-schema.name="pi" \
      org.label-schema.description="Calculate digits of pi" \
      org.label-schema.url="https://github.com/memes/pi" \
      org.label-schema.license="MIT"
COPY pi /pi
ENTRYPOINT ["/pi"]
CMD ["server"]
