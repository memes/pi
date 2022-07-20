FROM alpine:3.16.1 as ca
RUN apk --no-cache add ca-certificates-bundle=20211220-r0

FROM scratch
COPY --from=ca /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY pi /pi
EXPOSE 8080
EXPOSE 9090
ENTRYPOINT ["/pi"]
CMD ["server"]
