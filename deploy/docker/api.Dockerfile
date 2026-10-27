# syntax=docker/dockerfile:1.7
# One image for every backend process: api, worker and migrate (and later seed).

FROM golang:1.26-alpine AS build
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY backend/ ./

ARG VERSION=0.1.0-dev
ARG COMMIT=dev
ARG BUILT_AT=unknown
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath \
      -ldflags "-s -w \
        -X github.com/yelnurq/startify/backend/internal/platform/buildinfo.Version=${VERSION} \
        -X github.com/yelnurq/startify/backend/internal/platform/buildinfo.Commit=${COMMIT} \
        -X github.com/yelnurq/startify/backend/internal/platform/buildinfo.BuiltAt=${BUILT_AT}" \
      -o /out/ ./cmd/...

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/ /app/
USER nonroot:nonroot
EXPOSE 8080 8081
CMD ["/app/api"]
