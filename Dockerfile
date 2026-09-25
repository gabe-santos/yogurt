# The Server image: the one binary with the Web App embedded, as `make build` makes it.

# The SPA is static files, identical for every target, so it builds once on the
# build machine's own platform.
FROM --platform=$BUILDPLATFORM node:22-alpine AS web
RUN npm install --global pnpm@11
WORKDIR /src/web
COPY web/package.json web/pnpm-lock.yaml web/.npmrc ./
RUN pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm build

# SQLite is pure Go, so the binary cross-compiles without cgo.
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS server
ARG TARGETOS TARGETARCH
ARG VERSION=dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/
COPY --from=web /src/internal/webui/dist/spa/ internal/webui/dist/spa/
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -ldflags "-X github.com/gabe-santos/yogurt/internal/version.Version=$VERSION" -o /yogurt ./cmd/yogurt
RUN mkdir /data

# distroless/static carries CA certificates for HTTPS Feeds, and runs as an
# unprivileged user that must own the data directory.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=server /yogurt /yogurt
COPY --from=server --chown=nonroot:nonroot /data /data
COPY THIRD_PARTY_NOTICES /usr/share/doc/yogurt/THIRD_PARTY_NOTICES
ENV YOGURT_DATA_DIR=/data
EXPOSE 8080
ENTRYPOINT ["/yogurt"]
