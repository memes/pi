FROM alpine:3.21.2 as ca
RUN apk --no-cache add ca-certificates-bundle=20241121-r1

FROM scratch
COPY --from=ca /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY pi /pi
EXPOSE 8443
ENTRYPOINT ["/pi"]
CMD ["server"]
