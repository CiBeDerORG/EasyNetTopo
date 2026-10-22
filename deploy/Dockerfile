ARG RUST_VERSION=1.95
ARG NODE_VERSION=22
FROM node:${NODE_VERSION}-bookworm-slim AS node

FROM rust:${RUST_VERSION}-bookworm
ARG RUST_VERSION
ARG PNPM_VERSION=10.34.7
ARG PROTOC_VERSION=35.1

# Only the toolchain is baked into this image. Source, secrets and app data
# never enter the build context; deploy.sh uses deploy/ as the context.
COPY --from=node /usr/local/bin/node /usr/local/bin/node
COPY --from=node /usr/local/lib/node_modules /usr/local/lib/node_modules

# Bookworm bases may list Debian's official repositories over HTTP. With some
# HTTP proxy routes this fails with 502 before APT can verify InRelease.
# Keep Signed-By and apt-secure intact; use official HTTPS endpoints instead.
RUN set -eu; \
    for source in /etc/apt/sources.list /etc/apt/sources.list.d/*.sources /etc/apt/sources.list.d/*.list; do \
        [ -f "$source" ] || continue; \
        sed -i \
            -e 's|http://deb.debian.org/|https://deb.debian.org/|g' \
            -e 's|http://security.debian.org/|https://security.debian.org/|g' \
            "$source"; \
    done

RUN ln -s ../lib/node_modules/npm/bin/npm-cli.js /usr/local/bin/npm \
    && apt-get -o Acquire::Retries=3 update \
    && apt-get -o Acquire::Retries=3 install -y --no-install-recommends \
       build-essential ca-certificates clang cmake curl git libclang-dev \
       libssl-dev mold pkg-config python3 rsync unzip \
    && rm -rf /var/lib/apt/lists/* \
    && npm install --global "pnpm@${PNPM_VERSION}" \
    && npm cache clean --force \
    && rustup toolchain install "${RUST_VERSION}" --profile minimal

RUN set -eu; \
    case "$(dpkg --print-architecture)" in \
      amd64) protoc_arch=x86_64 ;; \
      arm64) protoc_arch=aarch_64 ;; \
      *) echo 'This development image supports Linux amd64 and arm64.' >&2; exit 1 ;; \
    esac; \
    curl --fail --location --retry 3 \
      "https://github.com/protocolbuffers/protobuf/releases/download/v${PROTOC_VERSION}/protoc-${PROTOC_VERSION}-linux-${protoc_arch}.zip" \
      --output /tmp/protoc.zip; \
    unzip -q /tmp/protoc.zip -d /usr/local; \
    rm /tmp/protoc.zip; \
    test "$(protoc --version)" = "libprotoc ${PROTOC_VERSION}"

ENV RUSTUP_TOOLCHAIN=${RUST_VERSION} \
    PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1
WORKDIR /data
