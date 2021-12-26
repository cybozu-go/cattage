# Build the manager binary
FROM quay.io/cybozu/golang:1.17-focal as builder

COPY ./ .
RUN CGO_ENABLED=0 go build -ldflags="-w -s" -o cattage-controller ./cmd/cattage-controller

# the controller image
FROM scratch
LABEL org.opencontainers.image.source https://github.com/cybozu-go/cattage

COPY --from=builder /work/cattage-controller ./
USER 10000:10000

ENTRYPOINT ["/cattage-controller"]
