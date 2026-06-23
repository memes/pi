FROM alpine:3.24.1 AS ca
RUN apk --no-cache add ca-certificates-bundle=20260611-r0

FROM scratch
COPY --from=ca /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
ARG TARGETPLATFORM
COPY $TARGETPLATFORM/pi /pi
EXPOSE 8443
ENTRYPOINT ["/pi"]
CMD ["server"]
