FROM scratch
LABEL org.opencontainers.image.authors="Cybozu, Inc." \
      org.opencontainers.image.title="cattage" \
      org.opencontainers.image.source="https://github.com/cybozu-go/cattage"
WORKDIR /
COPY LICENSE /
COPY cattage-controller /
USER 65532:65532

ENTRYPOINT ["/cattage-controller"]
