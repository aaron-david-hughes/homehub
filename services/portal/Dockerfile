# Portal image. Every service has its own Dockerfile like this one; to add a
# service, copy it and change the SERVICE default.
#
# The build CONTEXT must be the repo root (so the build can see shared/ and
# go.work), while the Dockerfile lives with the service:
#
#   docker build -f services/portal/Dockerfile -t homehub/portal:dev .
#
# The Go toolchain always runs on the build machine and cross-compiles for the
# target platform, so building linux/arm64 images on an amd64 CI runner (or an
# Apple Silicon Mac building for the Pi) is fast and needs no emulation.

FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS build
ARG TARGETOS
ARG TARGETARCH
ARG SERVICE=portal
ARG VERSION=dev

WORKDIR /src
# go.work lists every service, so the whole services/ tree must be present.
COPY go.work ./
COPY shared ./shared
COPY services ./services

RUN test -n "${SERVICE}" || (echo "build-arg SERVICE is required" && exit 1)
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" \
    -o /out/app ./services/${SERVICE}/cmd/${SERVICE}

# Distroless static: no shell or package manager, includes CA certs and tzdata.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/app /app
EXPOSE 8080
ENTRYPOINT ["/app"]
