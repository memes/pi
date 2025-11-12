FROM alpine:3.22.2 as ca
RUN apk --no-cache add ca-certificates-bundle=20250911-r0

FROM scratch
COPY --from=ca /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY pi /pi
EXPOSE 8443
ENTRYPOINT ["/pi"]
CMD ["server"]
