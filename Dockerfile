FROM alpine:3.17.1 as ca
RUN apk --no-cache add ca-certificates-bundle=20220614-r4

FROM scratch
COPY --from=ca /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY pi /pi
EXPOSE 8443
ENTRYPOINT ["/pi"]
CMD ["server"]
