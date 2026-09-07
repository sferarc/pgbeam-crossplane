# Controller image for the PgBeam Crossplane provider.
# Built and pushed to ghcr.io/sferarc/provider-pgbeam-controller by
# release-providers.yml. That is a different image from the provider package:
# the xpkg at ghcr.io/sferarc/provider-pgbeam only references this one, by the
# `controller.image` in package/crossplane.yaml.
FROM golang:1.27 AS build

WORKDIR /src

# Cache deps first. The mirror build is self-contained (the local replace
# directive on go.pgbeam.com/sdk is stripped by rewrite-for-mirror.sh).
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TARGETOS=linux
ARG TARGETARCH=amd64
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/provider .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/provider /usr/local/bin/provider
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/provider"]
