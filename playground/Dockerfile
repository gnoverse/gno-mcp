# syntax=docker/dockerfile:1

# ---- builder: compile static gnomcp + gnodev, matched to go.mod's pinned gno
FROM golang:1.26-bookworm AS builder
ENV CGO_ENABLED=0
WORKDIR /src
# Module graph first for layer caching
COPY go.mod go.sum ./
RUN go mod download
# gnomcp source + build
COPY . .
RUN go build -trimpath -o /out/gnomcp ./cmd/gnomcp
RUN go build -trimpath -o /out/simnet ./playground/e2e/simnet
RUN go build -trimpath -o /out/gnoquery ./playground/e2e/cmd/gnoquery
# gnodev from the gno commit pinned in go.mod. contribs/gnodev is a NESTED module
# (own go.mod with `replace => ../../`), so it is excluded from the parent module zip
# and `go install …@version` rejects it — clone the repo at the pinned commit instead.
RUN set -eux; \
    GNO_VER="$(go list -m -f '{{.Version}}' github.com/gnolang/gno)"; \
    GNO_COMMIT="${GNO_VER##*-}"; \
    git clone --filter=blob:none https://github.com/gnolang/gno /build/gno; \
    git -C /build/gno checkout "$GNO_COMMIT"; \
    (cd /build/gno/contribs/gnodev && go build -trimpath -o /out/gnodev .); \
    (cd /build/gno && go build -trimpath -o /out/gno ./gnovm/cmd/gno); \
    (cd /build/gno && go build -trimpath -o /out/gnokey ./gno.land/cmd/gnokey); \
    rm -rf /build/gno/.git

# ---- L1: fresh — clean Claude Code only (RUN TARGET: l1-fresh)
FROM node:22-bookworm-slim AS l1-fresh
# git+ca-certs for `/plugin marketplace add`; emacs-nox/vim/tmux/curl for live-demo
# ergonomics (curl also fetches starship below); zsh = default shell
RUN apt-get update && apt-get install -y --no-install-recommends \
        git ca-certificates emacs-nox vim tmux curl zsh \
    && rm -rf /var/lib/apt/lists/*
RUN npm install -g @anthropic-ai/claude-code
# starship prompt (install script auto-detects arch → /usr/local/bin)
RUN curl -sS https://starship.rs/install.sh | sh -s -- -y
RUN useradd --create-home --shell /usr/bin/zsh dev \
    && mkdir -p /home/dev/work \
    && printf '%s\n' 'eval "$(starship init zsh)"' > /home/dev/.zshrc \
    && chown -R dev:dev /home/dev
# Pre-complete the first-run wizard (welcome / theme picker / trust-folder) so an
# interactive `claude` with a valid CLAUDE_CODE_OAUTH_TOKEN drops straight to the
# prompt. Auth comes from the token at runtime; this only suppresses the UI gate.
# settings.json defaults to bypassPermissions so the container never prompts before
# tool calls (Claude still shows a one-time "accept" on launch — no config suppresses
# that, see GitHub claude-code#52501 — but no per-action prompts thereafter).
# sonnet/medium pins a cheap, consistent AUT for e2e; harmless default for the demo layers.
RUN mkdir -p /home/dev/.claude \
    && printf '%s\n' '{"hasCompletedOnboarding":true,"theme":"dark","projects":{"/home/dev/work":{"hasTrustDialogAccepted":true}}}' \
      > /home/dev/.claude.json \
    && printf '%s\n' '{"model":"sonnet","effortLevel":"medium","permissions":{"defaultMode":"bypassPermissions"}}' \
      > /home/dev/.claude/settings.json \
    && chown -R dev:dev /home/dev/.claude /home/dev/.claude.json
COPY playground/entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh
# UTF-8 locale so tmux renders starship's multibyte/Nerd-Font glyphs correctly
# (without it the container is POSIX/ASCII and tmux garbles them). C.UTF-8 is
# built into glibc — no locale-gen needed.
ENV LANG=C.UTF-8
ENV HOME=/home/dev
ENV PLAYGROUND_LAYER=l1-fresh
USER dev
WORKDIR /home/dev/work
ENTRYPOINT ["entrypoint.sh"]
CMD ["zsh"]

# ---- L2: gnomcp pre-installed, no local node (RUN TARGET: l2-gnomcp)
FROM l1-fresh AS l2-gnomcp
COPY --from=builder /out/gnomcp /usr/local/bin/gnomcp
# Keep a slim plugin/marketplace source in the image (just the plugin's own files,
# not the whole repo) so the marketplace resolves at runtime. Marketplace name
# marketplace "gnoverse" + plugin "gnomcp" come from .claude-plugin/marketplace.json.
COPY --chown=dev:dev .claude-plugin /opt/gnomcp/.claude-plugin
COPY --chown=dev:dev skills         /opt/gnomcp/skills
COPY --chown=dev:dev agents         /opt/gnomcp/agents
COPY --chown=dev:dev --chmod=755 hooks /opt/gnomcp/hooks
RUN claude plugin marketplace add /opt/gnomcp \
    && claude plugin install gnomcp@gnoverse --scope user \
    && claude mcp add gnomcp --scope user -- gnomcp
ENV PLAYGROUND_LAYER=l2-gnomcp

# ---- L3: full — adds gno + gnodev + GNOROOT (RUN TARGET: l3-full)
# gno (the CLI) enables local `gno test` / filetests against realm source with no
# chain and no keys — the inner loop the gno-build skill authors against.
FROM l2-gnomcp AS l3-full
COPY --from=builder /out/gno /usr/local/bin/gno
COPY --from=builder /out/gnodev /usr/local/bin/gnodev
COPY --from=builder /build/gno /build/gno
ENV GNOROOT=/build/gno
ENV PLAYGROUND_LAYER=l3-full

# ---- E2E: simnet-backed harness target — RUN VIA playground/e2e/scripts/up.sh
# The simulated testnet (node + faucet + gnoweb) runs from boot as the main
# process. gnodev stays agent-started (never automatic). Realms are baked
# twice: /opt/e2e/realms feeds simnet's genesis; /home/dev/realms is workspace
# material for agent-started-gnodev scenarios.
FROM l3-full AS e2e
COPY --from=builder /out/simnet /usr/local/bin/simnet
COPY --from=builder /out/gnoquery /usr/local/bin/gnoquery
COPY --from=builder /out/gnokey /usr/local/bin/gnokey
RUN mkdir -p /home/dev/.config/gnomcp
COPY --chown=dev:dev test/e2e/realms /opt/e2e/realms
COPY --chown=dev:dev test/e2e/realms /home/dev/realms
COPY --chown=dev:dev playground/e2e/profiles.e2e.toml /home/dev/.config/gnomcp/profiles.toml
ENV PLAYGROUND_LAYER=e2e
CMD ["simnet"]

# ---- SIM: e2e image for humans (RUN TARGET: sim, via `make playground-sim`)
# simnet starts as a background service from the init entrypoint; you land in
# an interactive shell with the chain already serving. The make target
# publishes gnoweb to the host (GNOWEB_PORT, default 8688).
FROM e2e AS sim
COPY --chmod=755 playground/sim-init.sh /usr/local/bin/sim-init.sh
ENV PLAYGROUND_LAYER=sim
ENTRYPOINT ["sim-init.sh"]
CMD ["zsh"]

# ---- SIM-CLA: sim image with the test13 CLA deploy gate seeded (RUN TARGET: sim-cla)
# Replicates the live-network CLA gate locally: simnet seeds gno.land/r/sys/cla
# (render/addrset deps from GNOROOT/examples) and activates it with a required hash,
# so a deploy must sign the CLA first — the local twin of scenario 14's deploy-gates.
# The faucet drips 10 GNOT; with the gas fee at the chain minimum that clears the
# whole Sign+deploy+bump flow with room to spare (raising the drip was never the fix
# — the over-provisioned fee was). Override the drip with `make sim-cla SIM_CLA_DRIP=…`.
FROM sim AS sim-cla
COPY --chown=dev:dev playground/e2e/simnet/realms /opt/e2e/cla-realms
ENV SIMNET_CLA_ARGS="-cla-dir /opt/e2e/cla-realms/gno.land/r/sys/cla -cla-hash e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
ENV SIMNET_GRANT=10000000
ENV PLAYGROUND_LAYER=sim-cla
