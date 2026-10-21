#!/usr/bin/env bash
# Private helper, invoked only by the repository-root deploy.sh.
# Reuse the existing EasyADB Buildx proxy settings without printing secrets,
# changing Docker daemon settings, or recreating another project's builder.
set -Eeuo pipefail

fail() { printf '错误：%s\n' "$*" >&2; exit 1; }
(( $# == 2 )) || fail '构建助手需要 Dockerfile 所在目录和镜像标签。'
context=$1
image_tag=$2
[[ -f "$context/Dockerfile" ]] || fail 'Dockerfile 不存在。'
command -v docker >/dev/null || fail '找不到 Docker CLI。'
docker buildx version >/dev/null 2>&1 || fail '需要 Docker Buildx。'

builder=easyadb-proxy
builder_container=buildx_buildkit_easyadb-proxy0
mihomo_container=mihomo

docker buildx inspect "$builder" >/dev/null 2>&1 ||
    fail '找不到 easyadb-proxy Buildx 构建器。'
docker container inspect "$builder_container" >/dev/null 2>&1 ||
    fail '找不到已有 BuildKit 容器 buildx_buildkit_easyadb-proxy0；请先检查 EasyADB 构建环境。'
[[ "$(docker inspect -f '{{with index .NetworkSettings.Networks "proxy-net"}}yes{{end}}' "$builder_container")" == yes ]] ||
    fail '已有 BuildKit 容器未连接 proxy-net，请先检查 EasyADB 构建环境。'
[[ "$(docker inspect -f '{{.State.Running}}' "$mihomo_container" 2>/dev/null)" == true ]] ||
    fail 'mihomo 容器未运行。'
mihomo_ip=$(docker inspect -f '{{with index .NetworkSettings.Networks "proxy-net"}}{{.IPAddress}}{{end}}' "$mihomo_container")
[[ "$mihomo_ip" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}$ ]] ||
    fail '未能取得 mihomo 在 proxy-net 上的 IPv4 地址。'

# The existing builder has authenticated HTTP(S)_PROXY from the EasyADB setup.
# Never print its .Config.Env, including via docker buildx inspect --bootstrap.
builder_proxy=$(
    docker inspect -f '{{range .Config.Env}}{{println .}}{{end}}' "$builder_container" |
        sed -n 's/^HTTPS_PROXY=//p' | head -n 1
) || fail '读取已有 BuildKit 代理配置失败。'
case "$builder_proxy" in
    http://mihomo:7890|http://*@mihomo:7890) ;;
    *) fail '已有 BuildKit 未配置预期的 Mihomo HTTPS_PROXY；请检查 EasyADB 的代理配置。' ;;
esac

# On fnOS, Buildx client-side registry token requests need the bridge address.
# Dockerfile RUN workers instead use the proxy-net hostname with --add-host.
host_proxy="${builder_proxy%mihomo:7890}${mihomo_ip}:7890"
printf '复用 easyadb-proxy：宿主认证和 Dockerfile 下载通过 Mihomo（不输出代理凭据）。\n'
HTTP_PROXY="$host_proxy" HTTPS_PROXY="$host_proxy" \
http_proxy="$host_proxy" https_proxy="$host_proxy" \
NO_PROXY='localhost,127.0.0.1' no_proxy='localhost,127.0.0.1' \
docker buildx build --builder "$builder" --load \
    --progress="${BUILDKIT_PROGRESS:-plain}" \
    --tag "$image_tag" \
    --file "$context/Dockerfile" \
    --add-host "mihomo:$mihomo_ip" \
    --build-arg "HTTP_PROXY=$builder_proxy" \
    --build-arg "HTTPS_PROXY=$builder_proxy" \
    --build-arg "http_proxy=$builder_proxy" \
    --build-arg "https_proxy=$builder_proxy" \
    --build-arg 'NO_PROXY=localhost,127.0.0.1' \
    --build-arg 'no_proxy=localhost,127.0.0.1' \
    "$context"
