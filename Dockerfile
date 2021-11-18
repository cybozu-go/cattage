# Build the manager binary
FROM quay.io/cybozu/golang:1.17-focal as builder

COPY ./ .
RUN CGO_ENABLED=0 go build -ldflags="-w -s" -o neco-tenant-controller ./cmd/neco-tenant-controller

# the controller image
FROM scratch
LABEL org.opencontainers.image.source https://github.com/cybozu-go/neco-tenant

COPY --from=builder /work/neco-tenant-controller ./
USER 10000:10000

ENTRYPOINT ["/neco-tenant-controller"]
