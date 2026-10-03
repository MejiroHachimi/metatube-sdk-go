FROM golang:1.26.8-alpine AS build
WORKDIR /src
ENV CGO_ENABLED=1 GOTOOLCHAIN=local GOMAXPROCS=4 GOFLAGS="-mod=readonly -p=4"
RUN --mount=type=secret,id=proxy_ca \
    if [ -f /run/secrets/proxy_ca ]; then export SSL_CERT_FILE=/run/secrets/proxy_ca; fi; \
    apk add --no-cache build-base libwebp-dev
COPY go.mod go.sum ./
RUN --mount=type=secret,id=proxy_ca \
    if [ -f /run/secrets/proxy_ca ]; then \
      export SSL_CERT_FILE=/run/secrets/proxy_ca; \
    fi; \
    go mod download
COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build go build -trimpath -ldflags="-linkmode=external -s -w -X github.com/metatube-community/metatube-sdk-go/internal/version.Version=0.1.0" -o /out/metatube ./cmd/metatube

FROM alpine:3.22 AS runtime
RUN --mount=type=secret,id=proxy_ca \
    if [ -f /run/secrets/proxy_ca ]; then export SSL_CERT_FILE=/run/secrets/proxy_ca; fi; \
    apk add --no-cache libwebp libwebpdemux && \
    ln -s libwebp.so.7 /usr/lib/libwebp.so && \
    ln -s libwebpdemux.so.2 /usr/lib/libwebpdemux.so && \
    mkdir -p /data && chown 10001:10001 /data
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/metatube /usr/local/bin/metatube
USER 10001:10001
ENV PORT=8080 DATA_DIR=/data GIN_MODE=release REQUIRE_NATIVE_WEBP=1 GOMEMLIMIT=192MiB
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/metatube"]

# Optional validation target: run the suite with native libwebp, including races.
FROM build AS test
RUN --mount=type=cache,target=/root/.cache/go-build REQUIRE_NATIVE_WEBP=1 go test -race ./internal/service ./imageutil ./database ./engine/dbengine

# Test-only image for reproducible HTTP/load validation; not the production target.
FROM build AS loadtest-tools
RUN --mount=type=cache,target=/root/.cache/go-build go build -ldflags="-linkmode=external" -o /out/eco-bench ./scripts/bench && \
    go build -o /out/test-fixtures ./scripts/fixtures

FROM runtime AS loadtest
COPY --from=loadtest-tools /out/eco-bench /out/test-fixtures /usr/local/bin/
COPY --chmod=755 scripts/loadtest-entrypoint.sh /usr/local/bin/loadtest-entrypoint.sh
ENTRYPOINT ["/bin/sh", "/usr/local/bin/loadtest-entrypoint.sh"]

FROM runtime AS production
