# syntax=docker/dockerfile:1
# Multi-stage container build for Alexandria server, client, CLI, and fake-connector images.

ARG NODE_IMAGE=node:22-alpine
ARG GO_IMAGE=golang:1.27-alpine
ARG RUNTIME_IMAGE=alpine:3.21

# Stage 1: Build the embedded React Console (/ui/) into internal/webui/dist.
FROM --platform=$BUILDPLATFORM ${NODE_IMAGE} AS gui-builder
RUN corepack enable
WORKDIR /src/gui
COPY gui/package.json gui/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY gui/ ./
RUN pnpm build:embed

# Stage 2: Cross-compile static Go binaries with the embedded Console (-tags gui).
FROM --platform=$BUILDPLATFORM ${GO_IMAGE} AS go-builder
ARG TARGETOS=linux
ARG TARGETARCH
ENV CGO_ENABLED=0 GOTOOLCHAIN=auto
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY deploy/fake-connector/ ./deploy/fake-connector/
COPY deploy/fake-idp/ ./deploy/fake-idp/
COPY internal/ ./internal/
COPY --from=gui-builder /src/internal/webui/dist ./internal/webui/dist
RUN mkdir -p /out && \
    GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -tags gui -o /out/alexandria-server ./cmd/alexandria-server && \
    GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -tags gui -o /out/alexandria-client ./cmd/alexandria-client && \
    GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -o /out/alexandriac ./cmd/alexandriac && \
    GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -o /out/fake-connector ./deploy/fake-connector

# Shared minimal non-root runtime base with CA bundle and curl for healthchecks.
FROM ${RUNTIME_IMAGE} AS runtime-base
RUN apk add --no-cache ca-certificates curl tzdata && \
    addgroup -g 65532 -S alexandria && \
    adduser -u 65532 -S -G alexandria -h /var/lib/alexandria alexandria && \
    mkdir -p /etc/alexandria /var/lib/alexandria/hub /var/lib/alexandria/client && \
    chown -R 65532:65532 /var/lib/alexandria

# Target: alexandria-server (Vocabulary Hub + embedded /ui/ Console).
FROM runtime-base AS server
COPY --from=go-builder /out/alexandria-server /usr/local/bin/alexandria-server
COPY deploy/vendor /etc/alexandria/vendor
USER 65532:65532
WORKDIR /var/lib/alexandria/hub
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/alexandria-server"]
CMD ["-config", "/etc/alexandria/server.yaml"]

# Target: alexandria-client (DSP Edge Sidecar / Validator + embedded /ui/ Console).
FROM runtime-base AS client
COPY --from=go-builder /out/alexandria-client /usr/local/bin/alexandria-client
COPY deploy/rules.seed.json /etc/alexandria/rules.seed.json
USER 65532:65532
WORKDIR /var/lib/alexandria/client
EXPOSE 9080 9081 9090
ENTRYPOINT ["/usr/local/bin/alexandria-client"]
CMD ["-config", "/etc/alexandria/client.yaml", "-state-dir", "/var/lib/alexandria/client"]

# Target: alexandriac (Offline Profile DSL Compiler CLI).
FROM runtime-base AS cli
COPY --from=go-builder /out/alexandriac /usr/local/bin/alexandriac
COPY deploy/vendor /etc/alexandria/vendor
USER 65532:65532
ENTRYPOINT ["/usr/local/bin/alexandriac"]

# Target: fake-connector (Simulated DSP Connector for the optional demo profile).
FROM runtime-base AS fake-connector
COPY --from=go-builder /out/fake-connector /usr/local/bin/fake-connector
USER 65532:65532
EXPOSE 8282
ENTRYPOINT ["/usr/local/bin/fake-connector"]
CMD ["-listen", "0.0.0.0:8282"]
