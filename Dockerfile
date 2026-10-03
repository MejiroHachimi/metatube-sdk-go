FROM golang:1.26.8-alpine AS build
WORKDIR /src
ENV CGO_ENABLED=0 GOTOOLCHAIN=local GOMAXPROCS=4 GOFLAGS="-mod=readonly -p=4"
COPY go.mod go.sum ./
ARG BUILD_EXTRA_CA
RUN if [ -n "$BUILD_EXTRA_CA" ]; then \
      cp /etc/ssl/certs/ca-certificates.crt /tmp/build-ca.pem; \
      printf '%s' "$BUILD_EXTRA_CA" | base64 -d >> /tmp/build-ca.pem; \
      export SSL_CERT_FILE=/tmp/build-ca.pem; \
    fi; \
    go mod download && rm -f /tmp/build-ca.pem
COPY . .
RUN go build -trimpath -ldflags="-s -w -X github.com/metatube-community/metatube-sdk-go/internal/version.Version=0.1.0" -o /out/metatube ./cmd/metatube

FROM alpine:3.22
RUN mkdir -p /data && chown 10001:10001 /data
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/metatube /usr/local/bin/metatube
USER 10001:10001
ENV PORT=8080 DATA_DIR=/data GIN_MODE=release
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/metatube"]
