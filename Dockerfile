FROM golang:alpine as builder
WORKDIR /go/src
EXPOSE 8080

# Install git first as it is required for go mod tidy
RUN apk add --no-cache git build-base

# Copy everything first
COPY go.mod go.sum ./
COPY warp.go server.go ./

# Tidy up dependencies (fixes missing go.sum error)
RUN go mod tidy

# Build binaries
RUN CGO_ENABLED=0 GOOS=linux \
    go build -a -installsuffix cgo -ldflags '-s' -o warp warp.go && \
    go build -a -installsuffix cgo -ldflags '-s' -o server server.go

FROM alpine:latest

# Copy binaries
COPY --from=builder /go/src/warp /usr/local/bin/
COPY --from=builder /go/src/server /usr/local/bin/
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

COPY entrypoint.sh   /usr/local/bin/

# Install runtime dependencies
RUN echo "http://dl-4.alpinelinux.org/alpine/edge/testing" >> /etc/apk/repositories \
    && apk add --no-cache bash curl wget wireguard-tools openresolv ip6tables \
    && chmod +x /usr/local/bin/entrypoint.sh

ENV         DAEMON_MODE                     false
ENV         PROXY_UP                        ""
ENV         PROXY_PORT                      "8080"
ENV         PROXY_USER                      ""
ENV         PROXY_PASS                      ""
ENV         WIREGUARD_UP                    ""
ENV         WIREGUARD_CONFIG                ""
ENV         WIREGUARD_INTERFACE_PRIVATE_KEY ""
ENV         WIREGUARD_INTERFACE_DNS         "1.1.1.1"
ENV         WIREGUARD_INTERFACE_ADDRESS     ""
ENV         WIREGUARD_PEER_PUBLIC_KEY       ""
ENV         WIREGUARD_PEER_ALLOWED_IPS      "0.0.0.0/0"
ENV         WIREGUARD_PEER_ENDPOINT         ""

ENTRYPOINT  [ "entrypoint.sh" ]
