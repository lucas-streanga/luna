# Claude Code sandbox — Fedora Asahi Remix (aarch64, 16K kernel pages), rootless Podman.
#
# Design notes:
#  - Everything comes from dnf. Fedora's aarch64 packages are built with >=16K page
#    alignment, so they map correctly under Asahi's 16K kernel. Do NOT curl third-party
#    prebuilt aarch64 binaries into this image — many statically link jemalloc for 4K
#    pages and abort at startup with "Unsupported system page size".
#  - Runs as a non-root user (uid/gid 1000) so `userns_mode: keep-id` in compose maps
#    cleanly onto the default Fedora host user and workspace files stay owned by you.

FROM registry.fedoraproject.org/fedora:42

# --- Base toolchain (add your Luna build deps here: llvm, clang, rust, etc.). ---
RUN dnf -y --setopt=install_weak_deps=False install \
        nodejs npm \
        git openssh-clients ca-certificates \
        ripgrep fd-find jq \
        gcc gcc-c++ make binutils \
        procps-ng less which shadow-utils \
    && dnf clean all

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

# --- Non-root user; uid/gid overridable to match your host user for keep-id. ---
ARG USER=dev
ARG UID=1000
ARG GID=1000
RUN groupadd -g "${GID}" "${USER}" \
    && useradd -m -u "${UID}" -g "${GID}" -s /bin/bash "${USER}" \
    && mkdir -p "/home/${USER}/.claude" \
    && chown -R "${UID}:${GID}" "/home/${USER}"

USER ${USER}
ENV HOME="/home/${USER}"
ENV CLAUDE_CONFIG_DIR="/home/${USER}/.claude"
WORKDIR /luna

# Safe default: permission prompts stay ON. Given the isolation you can opt into
# `claude --dangerously-skip-permissions` at runtime if you want unattended mode.
CMD ["claude"]
