FROM alpine:3.14.2 as ca
RUN apk --no-cache add ca-certificates-bundle=20191127-r5

FROM scratch
COPY --from=ca /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY pi /pi
EXPOSE 8080
EXPOSE 9090
USER nobody
ENTRYPOINT ["/pi"]
CMD ["server"]
