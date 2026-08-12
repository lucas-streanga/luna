# Claude Code sandbox — Fedora Asahi Remix (aarch64, 16K kernel pages), rootless Podman.
#
# Design notes:
#  - Everything comes from dnf. Fedora's aarch64 packages are built with >=16K page
#    alignment, so they map correctly under Asahi's 16K kernel. Do NOT curl third-party
#    prebuilt aarch64 binaries into this image — many statically link jemalloc for 4K
#    pages and abort at startup with "Unsupported system page size".
#  - The Go toolchain is the ONE deliberate exception to that rule, because Fedora 42 lags
#    (1.25.10) and the pin is spec-visible, not a convenience — see the Go block below. The
#    caution above is about third-party binaries that hardcode a 4K page size; upstream Go
#    is not in that class, and the -race smoke test proves the mapping rather than assuming it.
#  - Runs as a non-root user (uid/gid 1000) so `userns_mode: keep-id` in compose maps
#    cleanly onto the default Fedora host user and workspace files stay owned by you.

FROM registry.fedoraproject.org/fedora:42

# --- Base toolchain (add your Luna build deps here: llvm, clang, rust, etc.).
#     golangci-lint is the Tier-1 lint gate (claude-agent-plan §E); libfaketime is the clock
#     perturbation in testing-strategy §6-L4. Both float with Fedora — unlike Go itself,
#     neither version is load-bearing. NB Fedora ships golangci-lint v2, whose config schema
#     differs from v1: .golangci.yml needs `version: "2"` and a separate `formatters:` block. ---
RUN dnf -y --setopt=install_weak_deps=False install \
        nodejs npm \
        git openssh-clients ca-certificates \
        ripgrep fd-find jq \
        gcc gcc-c++ make binutils \
        procps-ng less which shadow-utils \
        golangci-lint libfaketime \
    && dnf clean all

# --- Go toolchain, pinned. This pin is DUAL-PURPOSE and therefore a design decision, not a
#     packaging detail: it is both the toolchain that builds the compiler and the language
#     floor of the Go the compiler EMITS (the emitted program is one Go module with a single
#     static go.mod — compiler §1.8). Bump it deliberately, never by drift.
#     Checksums are upstream's, so a corrupted or substituted tarball fails the build — which
#     also means GO_VERSION can only be overridden together with both checksums. ---
ARG GO_VERSION=1.26.5
ARG GO_SHA256_AMD64=5c2c3b16caefa1d968a94c1daca04a7ca301a496d9b086e17ad77bb81393f053
ARG GO_SHA256_ARM64=fe4789e92b1f33358680864bbe8704289e7bb5fc207d80623c308935bd696d49
RUN set -eux; \
    arch="$(rpm --eval %{_arch})"; \
    case "${arch}" in \
      x86_64)  goarch=amd64; sha="${GO_SHA256_AMD64}" ;; \
      aarch64) goarch=arm64; sha="${GO_SHA256_ARM64}" ;; \
      *) echo "unsupported arch: ${arch}" >&2; exit 1 ;; \
    esac; \
    curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-${goarch}.tar.gz" -o /tmp/go.tgz; \
    echo "${sha}  /tmp/go.tgz" | sha256sum -c -; \
    tar -C /usr/local -xzf /tmp/go.tgz; \
    rm -f /tmp/go.tgz
ENV PATH="/usr/local/go/bin:${PATH}"

# --- GOTOOLCHAIN=local. With `go 1.26` in go.mod, a mismatched local toolchain does NOT error:
#     Go silently downloads the matching one from proxy.golang.org on first build. That is a
#     network dependency inside a sandbox meant to be hermetic, re-paid on every fresh
#     container. Fail loudly instead — a version mismatch should be a build break, not a fetch. ---
ENV GOTOOLCHAIN=local

# --- Go smoke test, the same trick as the `claude --version` check below. A -race build proves
#     two things at once: the tarball's LOAD segments map under Asahi's 16K pages, and the race
#     detector actually links (gcc + the runtime/race syso), which testing-strategy §7 makes a
#     hard gate. Caches land in /tmp and are removed, so no root-owned cache bakes into the image. ---
RUN set -eux; \
    printf 'package main\nimport "fmt"\nfunc main() { fmt.Println("go toolchain ok") }\n' > /tmp/smoke.go; \
    env GOCACHE=/tmp/gocache GOPATH=/tmp/gopath go build -race -o /tmp/smoke /tmp/smoke.go; \
    /tmp/smoke; \
    go version; \
    rm -rf /tmp/smoke /tmp/smoke.go /tmp/gocache /tmp/gopath

# --- Stable machine-id. Claude Code fingerprints the device partly from /etc/machine-id;
#     without a fixed one it thinks it's a new machine on every run and forces re-login. ---
RUN printf '%s\n' "0123456789abcdef0123456789abcdef" > /etc/machine-id

# --- Claude Code. npm pulls the native linux-arm64 binary via an optional dependency.
#     Pin to the image and disable the background updater so the container is reproducible. ---
ENV DISABLE_AUTOUPDATER=1
RUN npm install -g @anthropic-ai/claude-code

# --- 16K-page smoke test. If Anthropic's arm64 binary had 4K-aligned LOAD segments it
#     would segfault here, turning a silent runtime crash into a loud build failure. ---
RUN claude --version

# --- Non-root user; uid/gid overridable to match your host user for keep-id.
#     go/ and .cache/go-build must exist HERE, dev-owned: compose mounts a named volume over
#     each, and Podman's copy-up seeds an empty volume from the image dir — ownership included.
#     Without these lines the volumes arrive root-owned and every `go build` dies with
#     permission denied under keep-id. Same reason .claude is pre-created. ---
ARG USER=dev
ARG UID=1000
ARG GID=1000
RUN groupadd -g "${GID}" "${USER}" \
    && useradd -m -u "${UID}" -g "${GID}" -s /bin/bash "${USER}" \
    && mkdir -p "/home/${USER}/.claude" \
                "/home/${USER}/go/bin" \
                "/home/${USER}/.cache/go-build" \
    && chown -R "${UID}:${GID}" "/home/${USER}"

USER ${USER}
ENV HOME="/home/${USER}"
ENV CLAUDE_CONFIG_DIR="/home/${USER}/.claude"

# --- Go env. GOPATH/GOCACHE are Go's own defaults spelled out rather than left implicit,
#     because compose mounts a named volume at each path: if a future Go release moves a
#     default, the mounts must move with it, and that is easier to notice stated than inferred.
#     GOFLAGS is deliberately NOT set image-wide — claude-agent-plan §A.2 scopes `-mod=vendor`
#     to the test-runner service, and setting it here would break non-module invocations. ---
ENV GOPATH="/home/${USER}/go"
ENV GOCACHE="/home/${USER}/.cache/go-build"
ENV PATH="/home/${USER}/go/bin:${PATH}"

WORKDIR /luna

# Safe default: permission prompts stay ON. Given the isolation you can opt into
# `claude --dangerously-skip-permissions` at runtime if you want unattended mode.
CMD ["claude"]
