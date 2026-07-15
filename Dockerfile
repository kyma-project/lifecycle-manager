# Build the manager binary
FROM --platform=$BUILDPLATFORM golang:1.26.5-alpine@sha256:0178a641fbb4858c5f1b48e34bdaabe0350a330a1b1149aabd498d0699ff5fb2 AS builder

WORKDIR /lifecycle-manager
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum

# Copy the go source
COPY cmd cmd/
COPY api api/
COPY maintenancewindows maintenancewindows/
COPY internal internal/
COPY pkg pkg/
COPY skr-webhook skr-webhook/
RUN chmod 755 skr-webhook/

# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Build
# TAG_default_tag comes from image builder: https://github.com/kyma-project/test-infra/tree/main/cmd/image-builder
ARG TAG_default_tag=from_dockerfile
ARG TARGETOS
ARG TARGETARCH

RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GOFIPS140=v1.0.0 go build -ldflags="-X 'main.buildVersion=${TAG_default_tag}'" -a -o manager cmd/main.go


# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot@sha256:f7f8f729987ad0fdf6b05eeeae94b26e6a0f613bdf46feea7fc40f7bd72953e6
WORKDIR /

COPY --chown=65532:65532 --from=builder /lifecycle-manager/manager .
COPY --chown=65532:65532 --from=builder /lifecycle-manager/skr-webhook skr-webhook/

USER 65532:65532

ENTRYPOINT ["/manager"]
