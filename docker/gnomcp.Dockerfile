# Release image for gnomcp. goreleaser builds the static binary (CGO_ENABLED=0)
# and places it in the build context; this only packages it. Build via the
# release pipeline, not `docker build` directly.
#
# gnomcp speaks MCP over stdio — run it attached:
#   docker run -i --rm ghcr.io/gnoverse/gnomcp:latest
# distroless/static carries CA certificates (needed for HTTPS RPC) and runs as
# nonroot. State (audit log, sessions, agent keys) lives under $HOME and is
# ephemeral unless a volume is mounted at /home/nonroot/.local/share/gnomcp.
FROM gcr.io/distroless/static-debian12:nonroot
# buildx provides TARGETOS/TARGETARCH; goreleaser lays each platform's binary
# under <os>/<arch>/ in the build context for multi-platform images.
ARG TARGETOS
ARG TARGETARCH
COPY ${TARGETOS}/${TARGETARCH}/gnomcp /usr/local/bin/gnomcp
ENTRYPOINT ["/usr/local/bin/gnomcp"]
