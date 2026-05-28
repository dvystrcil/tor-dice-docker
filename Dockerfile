# Multi-stage build for tor-dice:
#
#   1. web-build:  Node 22 alpine, install npm deps, vite build → web/dist
#   2. go-build:   golang:1.26 alpine, ingest web/dist via //go:embed,
#                  compile static binary (CGO_ENABLED=0)
#   3. final:      scratch (no shell, no libc) — just the binary
#
# Result: ~15 MiB image, single static-linked binary that contains
# both the Svelte SPA and the Go server. No nginx; no separate config
# files.

# ---- 1. Web build ----
FROM node:22-alpine AS web-build
WORKDIR /web
COPY web/package.json web/package-lock.json* ./
RUN --mount=type=cache,target=/root/.npm \
    npm ci --prefer-offline --no-audit --no-fund 2>/dev/null || \
    npm install --prefer-offline --no-audit --no-fund
COPY web/ ./
RUN npm run build

# ---- 2. Go build ----
FROM golang:1.26-alpine AS go-build
WORKDIR /src
COPY go.mod go.sum* ./
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go mod download
COPY . .
# Replace any local web/dist with the freshly-built version.
RUN rm -rf web/dist
COPY --from=web-build /web/dist ./web/dist
ARG VERSION=dev
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=linux \
    go build \
        -ldflags="-s -w -X main.version=${VERSION}" \
        -trimpath \
        -o /out/tor-dice \
        .

# ---- 3. Final ----
FROM scratch
COPY --from=go-build /out/tor-dice /tor-dice
EXPOSE 8080
ENTRYPOINT ["/tor-dice"]
