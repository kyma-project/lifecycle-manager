# Build the manager binary
FROM golang:1.22.2-alpine as builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum

# Copy the go source
COPY cmd cmd/
COPY api api/
COPY internal internal/
COPY pkg pkg/
COPY skr-webhook skr-webhook/
RUN chmod 755 skr-webhook

# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Build
# TAG_default_tag comes from image builder: https://github.com/kyma-project/test-infra/tree/main/cmd/image-builder
ARG TAG_default_tag=from_dockerfile

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-X 'main.buildVersion=${TAG_default_tag}'" -a -o manager cmd/main.go


# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static
WORKDIR /

COPY --from=builder /workspace/manager .
COPY --from=builder /workspace/skr-webhook skr-webhook/

ENTRYPOINT ["/manager"]
