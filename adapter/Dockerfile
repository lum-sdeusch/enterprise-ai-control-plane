# syntax=docker/dockerfile:1.7
FROM golang:1.26-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
ARG VERSION=dev
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X github.com/lum-sdeusch/enterprise-ai-control-plane/adapter/internal/config.Version=${VERSION}" \
    -o /out/eacp-adapter ./cmd/eacp-adapter

# Distroless static, non-root, read-only root filesystem friendly.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/eacp-adapter /eacp-adapter
USER 65532:65532
EXPOSE 9001 8080
ENTRYPOINT ["/eacp-adapter"]
