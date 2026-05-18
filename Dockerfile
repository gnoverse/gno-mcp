# syntax=docker/dockerfile:1.7

# --- build ----------------------------------------------------------------
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/gno-mcp ./cmd/gno-mcp

# --- runtime --------------------------------------------------------------
FROM gcr.io/distroless/static:nonroot
WORKDIR /home/nonroot
COPY --from=build /out/gno-mcp /usr/local/bin/gno-mcp

# Session lives in memory by default — no volume needed for the happy path.
# To persist the session keypair across container restarts, mount a writable
# directory at /home/nonroot/.gno-mcp and set GNO_MCP_SESSION_FILE accordingly
# (the encrypted-file backing is on the v0.3 roadmap; v0.2 reads the env
# variables but only persists the audit log).
ENV GNO_MCP_NETWORK=staging.gno.land

# stdio MCP. No port exposed.
ENTRYPOINT ["/usr/local/bin/gno-mcp"]
