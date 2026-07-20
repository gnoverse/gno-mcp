# Release image for agentfaucet. goreleaser builds the static binary
# (CGO_ENABLED=0) and places it in the build context; this only packages it.
# Build via the release pipeline, not `docker build` directly.
#
# agentfaucet is an HTTP service. Its -listen default (127.0.0.1:8590) is a
# host-safety choice that does not serve outside the container — bind all
# interfaces and publish the port:
#   docker run --rm -p 8590:8590 -e GNOMCP_FAUCET_MNEMONIC=... \
#     ghcr.io/gnoverse/agentfaucet:latest \
#     -rpc-url https://rpc.test13.testnets.gno.land:443 -chain-id test-13 -listen 0.0.0.0:8590
# The funding mnemonic must arrive via env (GNOMCP_FAUCET_MNEMONIC), never argv.
FROM gcr.io/distroless/static-debian12:nonroot
# buildx provides TARGETOS/TARGETARCH; goreleaser lays each platform's binary
# under <os>/<arch>/ in the build context for multi-platform images.
ARG TARGETOS
ARG TARGETARCH
COPY ${TARGETOS}/${TARGETARCH}/agentfaucet /usr/local/bin/agentfaucet
EXPOSE 8590
ENTRYPOINT ["/usr/local/bin/agentfaucet"]
