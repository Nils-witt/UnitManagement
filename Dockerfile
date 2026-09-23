# Build stages run on the build host's platform; only the Go binary is
# cross-compiled for the target, so multi-arch builds need no emulation.
FROM --platform=$BUILDPLATFORM node:24-alpine AS frontend
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci
COPY frontend/ ./
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS backend
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
COPY --from=frontend /src/frontend/dist ./frontend/dist
ARG TARGETOS TARGETARCH
ARG VERSION=
ARG COMMIT=unknown
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w -X go-unit-mangement/internal/version.Version=${VERSION} -X go-unit-mangement/internal/version.Commit=${COMMIT}" -o /server .

FROM gcr.io/distroless/static-debian12
COPY --from=backend /server /server
EXPOSE 8080
ENTRYPOINT ["/server"]
