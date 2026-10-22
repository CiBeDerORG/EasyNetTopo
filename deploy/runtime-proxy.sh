#!/usr/bin/env bash
# Source from deploy.sh for BUILDX_BUILDER=easyadb-proxy.
# Read existing BuildKit proxy credentials without logging or storing them.
set -Eeuo pipefail
runtime_proxy_fail() { printf '错误：运行容器代理初始化失败：%s\n' "$*" >&2; exit 1; }

[[ "${BUILDX_BUILDER:-}" == easyadb-proxy ]] ||
    runtime_proxy_fail '需要 easyadb-proxy 构建器。'
docker network inspect proxy-net >/dev/null 2>&1 ||
    runtime_proxy_fail '找不到现有 Docker 网络 proxy-net。'
docker container inspect buildx_buildkit_easyadb-proxy0 >/dev/null 2>&1 ||
    runtime_proxy_fail '找不到现有 BuildKit 容器 buildx_buildkit_easyadb-proxy0。'
[[ "$(docker inspect -f '{{with index .NetworkSettings.Networks "proxy-net"}}yes{{end}}' buildx_buildkit_easyadb-proxy0)" == yes ]] ||
    runtime_proxy_fail '已有 BuildKit 未连接 proxy-net。'
[[ "$(docker inspect -f '{{.State.Running}}' mihomo 2>/dev/null || :)" == true ]] ||
    runtime_proxy_fail 'mihomo 容器未运行。'
[[ "$(docker inspect -f '{{with index .NetworkSettings.Networks "proxy-net"}}yes{{end}}' mihomo)" == yes ]] ||
    runtime_proxy_fail 'mihomo 容器未连接 proxy-net。'

runtime_proxy_value=$(
    docker inspect -f '{{range .Config.Env}}{{println .}}{{end}}' buildx_buildkit_easyadb-proxy0 |
        sed -n 's/^HTTPS_PROXY=//p' | head -n 1
) || runtime_proxy_fail '无法读取已有 BuildKit 的 HTTPS_PROXY。'
case "$runtime_proxy_value" in
    http://mihomo:7890|http://*@mihomo:7890) ;;
    *) runtime_proxy_fail '已有 BuildKit 代理不是预期的 mihomo:7890。' ;;
esac
[[ "$runtime_proxy_value" != *$'\r'* && "$runtime_proxy_value" != *$'\n'* ]] ||
    runtime_proxy_fail '代理 URL 包含无效换行。'
export EASYTIER_RUNTIME_PROXY="$runtime_proxy_value"
unset runtime_proxy_value
