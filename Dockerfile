FROM golang:1.11 AS builder
LABEL maintainer="Ian Molee <imolee@gmail.com>"
LABEL repository="https://github.com/ianfoo/github-stargazer"

WORKDIR /go/src/github-stargazer
COPY . .
ENV GO111MODULE=on
RUN GO111MODULE=on CGO_ENABLED=0 go install -ldflags "-s" -v ./...

FROM alpine:latest AS certs
RUN apk --update add ca-certificates

FROM scratch
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /go/bin/github-stargazer /usr/local/bin/
EXPOSE 4040
ENTRYPOINT ["/usr/local/bin/github-stargazer"]
