FROM alpine:3.13.5 as ca
RUN apk --no-cache add ca-certificates-bundle=20191127-r5

FROM scratch
COPY --from=ca /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
EXPOSE 8080
EXPOSE 9090
COPY pi /pi
ENTRYPOINT ["/pi"]
CMD ["server"]
