FROM alpine:3.23.4 AS ca
RUN apk --no-cache add ca-certificates-bundle=20260413-r0

FROM scratch
COPY --from=ca /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
ARG TARGETPLATFORM
COPY $TARGETPLATFORM/pi /pi
EXPOSE 8443
ENTRYPOINT ["/pi"]
CMD ["server"]
