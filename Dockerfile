FROM golang:1.24-alpine AS build

WORKDIR /src

COPY go.mod ./

RUN go mod download

COPY cmd ./cmd

COPY internal ./internal

ARG TARGETOS

ARG TARGETARCH

RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api && \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker && \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/healthcheck ./cmd/healthcheck

FROM scratch AS api

ARG IMAGE_VERSION=dev

ARG VCS_REVISION=unknown

ARG BUILD_DATE=unknown

ARG SOURCE_URL=https://github.com/doryosi/go-learning-app

LABEL org.opencontainers.image.title="go-learning-app-api" \
      org.opencontainers.image.version="${IMAGE_VERSION}" \
      org.opencontainers.image.revision="${VCS_REVISION}" \
      org.opencontainers.image.created="${BUILD_DATE}" \
      org.opencontainers.image.source="${SOURCE_URL}"

COPY --from=build /out/api /api

COPY --from=build /out/healthcheck /healthcheck

USER 65532:65532

EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=2s --start-period=3s --retries=3 \
    CMD ["/healthcheck", "http://127.0.0.1:8080/healthz"]

ENTRYPOINT ["/api"]

FROM scratch AS worker

ARG IMAGE_VERSION=dev

ARG VCS_REVISION=unknown

ARG BUILD_DATE=unknown

ARG SOURCE_URL=https://github.com/doryosi/go-learning-app

LABEL org.opencontainers.image.title="go-learning-app-worker" \
      org.opencontainers.image.version="${IMAGE_VERSION}" \
      org.opencontainers.image.revision="${VCS_REVISION}" \
      org.opencontainers.image.created="${BUILD_DATE}" \
      org.opencontainers.image.source="${SOURCE_URL}"

COPY --from=build /out/worker /worker

USER 65532:65532

ENTRYPOINT ["/worker"]
