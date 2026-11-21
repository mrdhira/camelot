# Camelot's image: the binary cross-compiled on the build host (no
# emulation), on scratch with what distroless/static carries (CA bundle,
# tzdata, passwd/group with nonroot 65532, nsswitch.conf, /tmp).
# Build context = repository root (allow-listed by .dockerignore).
ARG GO_IMAGE=golang:1.27.4

FROM --platform=$BUILDPLATFORM ${GO_IMAGE} AS build
ARG TARGETOS
ARG TARGETARCH
ENV CGO_ENABLED=0 GOTOOLCHAIN=local GOFLAGS=-mod=readonly
RUN mkdir -p /rootfs/etc/ssl/certs /rootfs/usr/share /rootfs/home/nonroot /rootfs/tmp && \
    cp /etc/ssl/certs/ca-certificates.crt /rootfs/etc/ssl/certs/ && \
    cp -r /usr/share/zoneinfo /rootfs/usr/share/ && \
    printf 'root:x:0:0:root:/root:/sbin/nologin\nnonroot:x:65532:65532:nonroot:/home/nonroot:/sbin/nologin\n' > /rootfs/etc/passwd && \
    printf 'root:x:0:\nnonroot:x:65532:\n' > /rootfs/etc/group && \
    printf 'hosts: files dns\n' > /rootfs/etc/nsswitch.conf && \
    chown 65532:65532 /rootfs/home/nonroot && chmod 1777 /rootfs/tmp
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download
COPY services/ services/
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GOOS="$TARGETOS" GOARCH="$TARGETARCH" \
    go build -trimpath -ldflags "-s -w" -o /out/camelot ./services/camelot

FROM scratch AS runtime
ARG VERSION=unknown
ARG REVISION=unknown
LABEL org.opencontainers.image.title="camelot" \
      org.opencontainers.image.source="https://github.com/mrdhira/camelot" \
      org.opencontainers.image.version="$VERSION" \
      org.opencontainers.image.revision="$REVISION"
COPY --from=build /rootfs/ /
COPY --from=build /out/camelot /app/camelot
USER 65532:65532
ENTRYPOINT ["/app/camelot"]
CMD ["serve"]
