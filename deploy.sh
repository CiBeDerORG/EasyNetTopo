#!/usr/bin/env bash
# One public entry point. Deployment support lives in deploy/; local state in data/.
set -Eeuo pipefail
umask 077

fail() { printf '错误：%s\n' "$*" >&2; exit 1; }
info() { printf '%s\n' "$*"; }
usage() {
    cat <<'USAGE'
用法：./deploy.sh [up|update|check|init|stop|status|logs|schedule] [参数]
  up      默认操作；按 .env 选择分支、构建并启动，等待服务就绪
  update  停服务，按 .env 分支 git pull --ff-only，然后重新部署
  check   检查远端 Git 分支；文件树未变跳过，有更新才部署
  schedule start 15m|status|logs|stop   管理 cron 定时检查
  init    只准备目录与 .env；已有 .env 保持不变
  stop    停止 Compose，保留 data/ 的数据库、缓存和工作区
  status  查看容器和编译状态；服务未就绪时退出码非零
  logs    跟随日志，可指定 backend 或 frontend

可在父目录运行 ./EasyNetTopo/deploy.sh；仓库目录名可自定义。编译失败直接失败，不回滚。
USAGE
}

ACTION=${1:-up}
case "$ACTION" in -h|--help|help) usage; exit 0 ;; esac
case "$ACTION" in up|update|check|init|stop|status|logs|schedule) ;; *) usage >&2; exit 2 ;; esac
if [[ "$ACTION" == schedule ]]; then
    (( $# >= 2 && $# <= 3 )) || { usage >&2; exit 2; }
    schedule_repo=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)
    exec bash "$schedule_repo/deploy/scheduled-deploy.sh" "${@:2}"
fi
(( $# <= 2 )) || fail '参数过多；运行 ./deploy.sh --help 查看用法。'
if (( $# == 2 )); then
    [[ "$ACTION" == logs && ( "$2" == backend || "$2" == frontend ) ]] || fail '仅 logs 接受 backend 或 frontend 参数。'
fi

REPO_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)
ENV_FILE="$REPO_DIR/.env"
DATA_DIR="$REPO_DIR/data"
[[ "$REPO_DIR" != *$'\n'* && "$REPO_DIR" != *$'\r'* ]] || fail '仓库路径不能包含换行。'
[[ -f "$REPO_DIR/deploy/.env.example" && -f "$REPO_DIR/deploy/compose.yaml" ]] || fail '缺少 deploy/ 配套文件；请使用包含部署文件的分支。'
[[ ! -L "$ENV_FILE" ]] || fail '.env 必须是当前仓库中的普通文件，不能是符号链接。'
[[ ! -L "$DATA_DIR" ]] || fail 'data/ 不能是符号链接；移动整个仓库目录可更改部署总目录。'
mkdir -p -- "$DATA_DIR/run"

if [[ ! -e "$ENV_FILE" ]]; then
    # Noclobber also protects a file another invocation just created.
    ( set -o noclobber; cat "$REPO_DIR/deploy/.env.example" > "$ENV_FILE" ) 2>/dev/null || [[ -f "$ENV_FILE" ]]
    [[ -f "$ENV_FILE" ]] || fail '无法创建 .env。'
    info '已从 deploy/.env.example 创建 .env。'
fi
[[ -f "$ENV_FILE" && -r "$ENV_FILE" ]] || fail '.env 不是可读取的普通文件。'

# Parse data, never source/eval the file. Compose receives these exact values as
# exported variables, so '$' in a secret cannot be reinterpreted by .env expansion.
# Keep Docker's caller-supplied build overrides while loading repository defaults.
BUILDX_BUILDER_OVERRIDE=${BUILDX_BUILDER:-}
BUILDKIT_PROGRESS_OVERRIDE=${BUILDKIT_PROGRESS:-}
ENV_KEYS='GIT_BRANCH COMPOSE_PROJECT_NAME BUILDX_BUILDER BUILDKIT_PROGRESS RUST_VERSION NODE_VERSION PNPM_VERSION PROTOC_VERSION CARGO_BUILD_PROFILE CARGO_BUILD_JOBS EASYTIER_WATCH EASYTIER_POLL_INTERVAL EASYTIER_DEBOUNCE START_TIMEOUT WEB_BIND_ADDRESS WEB_PORT API_BIND_ADDRESS ET_API_SERVER_PORT CONFIG_BIND_ADDRESS ET_CONFIG_SERVER_PORT ET_CONFIG_SERVER_PROTOCOL ET_WEB_PUBLIC_ORIGIN ET_DISABLE_REGISTRATION ET_ALLOW_AUTO_CREATE_USER ET_WEBHOOK_URL ET_WEBHOOK_SECRET ET_INTERNAL_AUTH_TOKEN ET_WEB_INSTANCE_ID ET_WEB_INSTANCE_API_BASE_URL ET_GATEWAY_PEER_URL ET_GATEWAY_RELAY_DATA VITE_HMR_HOST VITE_HMR_PORT VITE_HMR_PROTOCOL VITE_ALLOWED_HOSTS'
for key in $ENV_KEYS; do unset "$key"; done
trim() {
    TRIMMED=${1#"${1%%[![:space:]]*}"}
    TRIMMED=${TRIMMED%"${TRIMMED##*[![:space:]]}"}
}
line_number=0
while IFS= read -r line || [[ -n "$line" ]]; do
    line_number=$((line_number + 1))
    line=${line%$'\r'}
    trim "$line"; line=$TRIMMED
    [[ -z "$line" || "$line" == \#* ]] && continue
    [[ "$line" =~ ^([A-Z_][A-Z0-9_]*)[[:space:]]*=(.*)$ ]] || fail ".env 第 $line_number 行格式无效；使用 KEY=value，注释放单独一行。"
    key=${BASH_REMATCH[1]}
    value=${BASH_REMATCH[2]}
    [[ " $ENV_KEYS " == *" $key "* ]] || fail ".env 第 $line_number 行配置项 $key 不受支持。"
    trim "$value"; value=$TRIMMED
    if [[ "$value" == \"* || "$value" == \'* ]]; then
        quote=${value:0:1}
        [[ ${#value} -ge 2 && "${value: -1}" == "$quote" ]] || fail ".env 第 $line_number 行引号未闭合。"
        value=${value:1:${#value}-2}
    fi
    [[ "$value" != *$'\r'* ]] || fail ".env 第 $line_number 行包含无效回车。"
    printf -v "$key" '%s' "$value"
    export "$key"
done < "$ENV_FILE"

BUILDX_BUILDER=${BUILDX_BUILDER_OVERRIDE:-${BUILDX_BUILDER:-}}
BUILDKIT_PROGRESS=${BUILDKIT_PROGRESS_OVERRIDE:-${BUILDKIT_PROGRESS:-auto}}
: "${GIT_BRANCH:=}" "${COMPOSE_PROJECT_NAME:=easynettopo-modding}"
: "${RUST_VERSION:=1.95}" "${NODE_VERSION:=22}" "${PNPM_VERSION:=10.34.7}" "${PROTOC_VERSION:=35.1}"
: "${CARGO_BUILD_PROFILE:=dev}" "${CARGO_BUILD_JOBS:=2}"
: "${EASYTIER_WATCH:=true}" "${EASYTIER_POLL_INTERVAL:=1}" "${EASYTIER_DEBOUNCE:=1}" "${START_TIMEOUT:=1800}"
: "${WEB_BIND_ADDRESS:=127.0.0.1}" "${WEB_PORT:=11210}" "${API_BIND_ADDRESS:=127.0.0.1}" "${ET_API_SERVER_PORT:=11211}"
: "${CONFIG_BIND_ADDRESS:=0.0.0.0}" "${ET_CONFIG_SERVER_PORT:=22020}" "${ET_CONFIG_SERVER_PROTOCOL:=udp}"
: "${ET_DISABLE_REGISTRATION:=false}" "${ET_ALLOW_AUTO_CREATE_USER:=false}" "${ET_GATEWAY_RELAY_DATA:=false}"
for key in $ENV_KEYS; do export "$key"; done

[[ "$COMPOSE_PROJECT_NAME" =~ ^[a-z0-9][a-z0-9_-]*$ ]] || fail 'COMPOSE_PROJECT_NAME 只能使用小写字母、数字、下划线与连字符。'
case "$BUILDKIT_PROGRESS" in
    auto|plain|tty|quiet|none|rawjson) ;;
    *) fail 'BUILDKIT_PROGRESS 只能是 auto、plain、tty、quiet、none 或 rawjson。' ;;
esac
for key in WEB_PORT ET_API_SERVER_PORT ET_CONFIG_SERVER_PORT; do
    value=${!key}
    [[ "$value" =~ ^[0-9]{1,5}$ ]] && (( 10#$value >= 1 && 10#$value <= 65535 )) || fail "$key 必须是 1–65535 的端口。"
    printf -v "$key" '%d' "$((10#$value))"
done
[[ "$WEB_PORT" != "$ET_API_SERVER_PORT" ]] || fail 'WEB_PORT 与 ET_API_SERVER_PORT 必须不同。'
for key in CARGO_BUILD_JOBS START_TIMEOUT; do
    value=${!key}
    [[ "$value" =~ ^[1-9][0-9]{0,5}$ ]] || fail "$key 必须是正整数。"
done
for key in EASYTIER_POLL_INTERVAL EASYTIER_DEBOUNCE; do
    [[ "${!key}" =~ ^([1-9][0-9]*([.][0-9]+)?|0[.][0-9]*[1-9][0-9]*)$ ]] || fail "$key 必须是大于零的秒数。"
done
for key in EASYTIER_WATCH ET_DISABLE_REGISTRATION ET_ALLOW_AUTO_CREATE_USER ET_GATEWAY_RELAY_DATA; do
    [[ "${!key}" == true || "${!key}" == false ]] || fail "$key 只能是 true 或 false。"
done
[[ "$CARGO_BUILD_PROFILE" == dev || "$CARGO_BUILD_PROFILE" == release ]] || fail 'CARGO_BUILD_PROFILE 只能是 dev 或 release。'
for key in RUST_VERSION NODE_VERSION PNPM_VERSION PROTOC_VERSION; do
    [[ "${!key}" =~ ^[0-9]+([.][0-9]+){0,2}$ ]] || fail "$key 必须是数字版本号。"
done
case "$ET_CONFIG_SERVER_PROTOCOL" in udp) CONFIG_TRANSPORT=udp ;; tcp|ws) CONFIG_TRANSPORT=tcp ;; *) fail 'ET_CONFIG_SERVER_PROTOCOL 只能是 udp、tcp 或 ws。' ;; esac
if [[ "$CONFIG_TRANSPORT" == tcp ]]; then
    [[ "$ET_CONFIG_SERVER_PORT" != "$WEB_PORT" && "$ET_CONFIG_SERVER_PORT" != "$ET_API_SERVER_PORT" ]] || fail 'TCP 配置服务不能与 Web/API 使用相同端口。'
fi
[[ -z "${VITE_HMR_PROTOCOL:-}" || "$VITE_HMR_PROTOCOL" == ws || "$VITE_HMR_PROTOCOL" == wss ]] || fail 'VITE_HMR_PROTOCOL 只能是 ws 或 wss。'
if [[ -n "${VITE_HMR_PORT:-}" ]]; then
    [[ "$VITE_HMR_PORT" =~ ^[0-9]{1,5}$ ]] && (( 10#$VITE_HMR_PORT >= 1 && 10#$VITE_HMR_PORT <= 65535 )) || fail 'VITE_HMR_PORT 必须是 1–65535 的端口。'
fi

if [[ "$ACTION" == init ]]; then
    info "仓库：$REPO_DIR"
    info "配置：$ENV_FILE"
    info "数据：$DATA_DIR"
    info "分支：${GIT_BRANCH:-当前检出分支}；Web 端口：$WEB_PORT"
    info "构建器：${BUILDX_BUILDER:-Docker 当前默认选择}；构建日志：$BUILDKIT_PROGRESS"
    exit 0
fi

# A no-change scheduled check must not require Docker connectivity.
if [[ "$ACTION" != check ]]; then
    command -v docker >/dev/null || fail '需要安装 Docker Engine 与 Docker Compose v2。'
    docker compose version >/dev/null 2>&1 || fail '需要 Docker Compose v2（docker compose）。'
    docker info >/dev/null 2>&1 || fail '无法连接 Docker daemon；请检查 Docker 服务及当前用户权限。'
fi
export EASYTIER_SOURCE_DIR="$REPO_DIR" EASYTIER_DATA_DIR="$DATA_DIR" CONFIG_TRANSPORT
export LOCAL_UID="$(id -u)" LOCAL_GID="$(id -g)"
# Use only the explicit config; never accidentally load another Compose project.
unset COMPOSE_FILE COMPOSE_PROFILES
COMPOSE=(docker compose --project-directory "$REPO_DIR" --env-file /dev/null --project-name "$COMPOSE_PROJECT_NAME" --file "$REPO_DIR/deploy/compose.yaml")

show_status() {
    "${COMPOSE[@]}" ps --all
    local service result=0
    for service in backend frontend; do
        if [[ -f "$DATA_DIR/run/$service.json" ]]; then
            printf '%s: ' "$service"
            cat "$DATA_DIR/run/$service.json"
            printf '\n'
        fi
        if ! "${COMPOSE[@]}" exec -T "$service" python3 /source/deploy/runner.py health "$service" >/dev/null 2>&1; then
            result=1
        fi
    done
    return "$result"
}

case "$ACTION" in
    status) show_status; exit $? ;;
    logs) exec "${COMPOSE[@]}" logs --follow --tail 100 "${@:2}" ;;
esac

command -v flock >/dev/null || fail '需要 flock（通常由 util-linux 提供），以防止重复部署。'
exec 9>"$DATA_DIR/run/deploy.lock"
if ! flock --nonblock 9; then
    if [[ "$ACTION" == check ]]; then
        info '已有部署正在执行，本轮未检查远端分支。'
        exit 75
    fi
    fail '已有部署命令在执行；请等待其完成。'
fi
cleanup() { rm -f -- "$DATA_DIR/run/pause"; }
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

if [[ "$ACTION" == stop ]]; then
    "${COMPOSE[@]}" down --remove-orphans
    info '已停止；data/ 保留。'
    exit 0
fi

command -v git >/dev/null || fail '需要 git。'
[[ "$(git -C "$REPO_DIR" rev-parse --show-toplevel 2>/dev/null)" == "$REPO_DIR" ]] || fail 'deploy.sh 必须位于项目 Git 仓库根目录；仓库目录名可自定义。'
origin=$(git -C "$REPO_DIR" remote get-url origin)
[[ "${origin,,}" =~ ^(https://github\.com/|git@github\.com:|ssh://git@github\.com/)cibederorg/easynettopo(\.git)?/?$ ]] || fail 'origin 必须指向 GitHub 上的 CiBeDerORG/EasyNetTopo（HTTPS 或 SSH）。'
current_branch=$(git -C "$REPO_DIR" symbolic-ref --quiet --short HEAD || true)
selected_branch=${GIT_BRANCH:-$current_branch}
[[ -n "$selected_branch" ]] || fail '当前为 detached HEAD；请在 .env 中填写 GIT_BRANCH。'
git check-ref-format --branch "$selected_branch" >/dev/null 2>&1 || fail 'GIT_BRANCH 不是有效的分支名。'
if [[ "$selected_branch" != "$current_branch" || "$ACTION" == update ]]; then
    [[ -z "$(git -C "$REPO_DIR" status --porcelain)" ]] || fail '切换分支或 update 前请先提交本地改动；脚本不会 reset 或 stash。'
fi

reexec_up() {
    [[ -f "$REPO_DIR/deploy.sh" && -f "$REPO_DIR/deploy/compose.yaml" ]] || fail '所选分支不包含部署文件；服务已停止，请选择已合入部署功能的分支。'
    cleanup
    trap - EXIT
    flock --unlock 9
    exec 9>&-
    # Re-read .env after an update; only the original caller may override it.
    if [[ -n "$BUILDX_BUILDER_OVERRIDE" ]]; then
        export BUILDX_BUILDER="$BUILDX_BUILDER_OVERRIDE"
    else
        unset BUILDX_BUILDER
    fi
    if [[ -n "$BUILDKIT_PROGRESS_OVERRIDE" ]]; then
        export BUILDKIT_PROGRESS="$BUILDKIT_PROGRESS_OVERRIDE"
    else
        unset BUILDKIT_PROGRESS
    fi
    exec bash "$REPO_DIR/deploy.sh" up
}


# Used by the scheduler. Only the checked-out branch / .env selected branch is
# eligible. Git runs as the original login user so root cron needs no SSH key.
# Do not stop Docker until a changed tree or a previous failed deployment is known.
if [[ "$ACTION" == check ]]; then
    git_user_uid=${EASYNETTOPO_GIT_UID:-${SUDO_UID:-$(id -u)}}
    [[ "$git_user_uid" =~ ^[0-9]{1,10}$ ]] || fail 'Git 用户 UID 无效。'
    git_as_owner() {
        if (( EUID == 0 )) && [[ "$git_user_uid" != 0 ]]; then
            sudo -n -H -u "#$git_user_uid" env GIT_TERMINAL_PROMPT=0 \
                GIT_SSH_COMMAND='ssh -o BatchMode=yes' \
                git -C "$REPO_DIR" "$@"
        else
            GIT_TERMINAL_PROMPT=0 GIT_SSH_COMMAND='ssh -o BatchMode=yes' \
                git -C "$REPO_DIR" "$@"
        fi
    }
    [[ "$(git_as_owner rev-parse --show-toplevel)" == "$REPO_DIR" ]] ||
        fail '原始 Git 用户无法访问该仓库根目录。'
    owner_origin=$(git_as_owner remote get-url origin)
    [[ "${owner_origin,,}" =~ ^(https://github\.com/|git@github\.com:|ssh://git@github\.com/)cibederorg/easynettopo(\.git)?/?$ ]] ||
        fail '原始 Git 用户的 origin 必须指向 CiBeDerORG/EasyNetTopo。'

    info "定时检查：正在拉取 origin/$selected_branch 的远端信息（尚未停止应用）。"
    git_as_owner fetch --no-tags origin "refs/heads/$selected_branch:refs/remotes/origin/$selected_branch"
    remote_tip=$(git_as_owner rev-parse --verify "refs/remotes/origin/$selected_branch")
    remote_tree=$(git_as_owner rev-parse "${remote_tip}^{tree}")

    # Preserve local changes, local commits and divergence: never reset/stash.
    owner_branch=$(git_as_owner symbolic-ref --quiet --short HEAD || :)
    if git_as_owner show-ref --verify --quiet "refs/heads/$selected_branch"; then
        local_target_tip=$(git_as_owner rev-parse "refs/heads/$selected_branch")
        git_as_owner merge-base --is-ancestor "$local_target_tip" "$remote_tip" ||
            fail "本地 $selected_branch 分支含有远端没有的提交或已分叉；拒绝自动覆盖。"
    else
        local_target_tip=
    fi
    if [[ "$owner_branch" != "$selected_branch" || "$local_target_tip" != "$remote_tip" ]]; then
        [[ -z "$(git_as_owner status --porcelain)" ]] ||
            fail '存在本地 Git 改动；为避免覆盖，定时更新已停止。请先人工处理。'
        if [[ "$owner_branch" != "$selected_branch" ]]; then
            if [[ -n "$local_target_tip" ]]; then
                git_as_owner switch "$selected_branch"
            else
                git_as_owner switch --create "$selected_branch" --track "origin/$selected_branch"
            fi
        fi
        if [[ "$local_target_tip" != "$remote_tip" && -n "$local_target_tip" ]]; then
            git_as_owner merge --ff-only "$remote_tip"
        fi
    fi
    active_tree=$(git_as_owner rev-parse 'HEAD^{tree}')
    [[ "$active_tree" == "$remote_tree" ]] || fail '本地分支树与远端不一致；未部署。'
    deployed_tree=$(cat "$DATA_DIR/run/deployed-git-tree" 2>/dev/null || :)
    if [[ "$deployed_tree" == "$remote_tree" && ! -e "$DATA_DIR/run/deploy-pending" ]]; then
        info '远端 Git 文件树没有变化；跳过 Docker 构建与服务重启。'
        exit 0
    fi
    # Even without remote changes, an untracked success marker may trigger a
    # retry. Never use cron to deploy arbitrary uncommitted local source.
    [[ -z "$(git_as_owner status --porcelain)" ]] ||
        fail '存在本地 Git 改动；拒绝定时构建，避免覆盖或部署未提交代码。'
    info '检测到 Git 文件树变化，或上次尚未成功部署；现在开始部署。'
    reexec_up
fi

# All build paths stop the old process first. A failed image/source build cannot
# silently leave the previously built application serving requests.
touch "$DATA_DIR/run/deploy-pending"
touch "$DATA_DIR/run/pause"
"${COMPOSE[@]}" stop
rm -f -- "$DATA_DIR/run/backend.json" "$DATA_DIR/run/frontend.json"

if [[ "$selected_branch" != "$current_branch" ]]; then
    if git -C "$REPO_DIR" show-ref --verify --quiet "refs/heads/$selected_branch"; then
        git -C "$REPO_DIR" switch "$selected_branch"
    else
        git -C "$REPO_DIR" fetch --no-tags origin "refs/heads/$selected_branch:refs/remotes/origin/$selected_branch"
        git -C "$REPO_DIR" switch --create "$selected_branch" --track "origin/$selected_branch"
    fi
    if [[ "$ACTION" != update ]]; then reexec_up; fi
fi
if [[ "$ACTION" == update ]]; then
    git -C "$REPO_DIR" pull --ff-only origin "$selected_branch"
    reexec_up
fi

# Record the exact clean source snapshot that is about to be built.
snapshot_head=$(git -C "$REPO_DIR" rev-parse HEAD)
snapshot_tree=$(git -C "$REPO_DIR" rev-parse 'HEAD^{tree}')
snapshot_clean=false
if [[ -z "$(git -C "$REPO_DIR" status --porcelain)" ]]; then
    snapshot_clean=true
fi
info "构建开发环境；源码分支：$selected_branch；构建器：${BUILDX_BUILDER:-Docker 当前默认选择}；构建日志：$BUILDKIT_PROGRESS"
# The dedicated EasyADB BuildKit has two proxy paths: the Buildx client needs
# a host-reachable address for Docker Hub tokens, while Dockerfile RUN stages
# use its proxy-net address. Reuse it without recreating/changing the builder.
if [[ "$BUILDX_BUILDER" == "easyadb-proxy" ]]; then
    bash "$REPO_DIR/deploy/build-mihomo.sh" "$REPO_DIR/deploy" "${COMPOSE_PROJECT_NAME}-dev:local"
else
    "${COMPOSE[@]}" build backend
fi
rm -f -- "$DATA_DIR/run/pause"
"${COMPOSE[@]}" up --detach --no-build --force-recreate --remove-orphans
info '正在安装依赖/增量编译；可在另一终端运行 deploy.sh logs 查看进度。'

deadline=$((SECONDS + START_TIMEOUT))
while (( SECONDS < deadline )); do
    ready=0
    for service in backend frontend; do
        if [[ -f "$DATA_DIR/run/$service.json" ]] && grep -Eq '"state"[[:space:]]*:[[:space:]]*"failed"' "$DATA_DIR/run/$service.json"; then
            fail "$service 构建或运行失败；运行 ./deploy.sh logs $service 查看原因。未回滚，修正后重新部署或等待源码监听重试。"
        fi
        container_id=$("${COMPOSE[@]}" ps --all --quiet "$service")
        [[ -n "$container_id" ]] || fail "$service 容器不存在；运行 ./deploy.sh logs 查看原因。"
        container_state=$(docker inspect --format '{{.State.Status}} {{if .State.Health}}{{.State.Health.Status}}{{end}}' "$container_id")
        case "$container_state" in
            'running healthy') ready=$((ready + 1)) ;;
            exited*|dead*|restarting*) fail "$service 容器未正常运行；请查看日志。" ;;
        esac
    done
    if (( ready == 2 )); then
        # Never advance the successful Git marker after a failed or dirty build.
        if [[ "$snapshot_clean" == true &&
              "$(git -C "$REPO_DIR" rev-parse HEAD)" == "$snapshot_head" &&
              -z "$(git -C "$REPO_DIR" status --porcelain)" ]]; then
            version_tmp="$DATA_DIR/run/deployed-git-tree.tmp.$$"
            printf '%s\n' "$snapshot_tree" > "$version_tmp"
            mv -f -- "$version_tmp" "$DATA_DIR/run/deployed-git-tree"
        else
            info '源码在构建期间变化或包含本地修改；不更新定时部署成功版本标记。'
        fi
        rm -f -- "$DATA_DIR/run/deploy-pending"
        info "部署就绪：Web http://$WEB_BIND_ADDRESS:$WEB_PORT"
        info "配置服务：$ET_CONFIG_SERVER_PROTOCOL / $ET_CONFIG_SERVER_PORT；数据：$DATA_DIR"
        exit 0
    fi
    sleep 1
done
fail "等待服务就绪超过 $START_TIMEOUT 秒；请查看 status/logs，编译可能仍在进行。"
