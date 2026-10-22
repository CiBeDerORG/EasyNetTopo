#!/usr/bin/env bash
# Managed cron schedule for the single public entry point: ../deploy.sh.
# No application files or Git credentials are stored in cron or this config.
set -Eeuo pipefail
umask 077

fail() { printf '错误：%s\n' "$*" >&2; exit 1; }
repo_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)
state_dir="$repo_dir/data/run"
config="$state_dir/scheduled.conf"
last_check="$state_dir/scheduled-last-check"
last_result="$state_dir/scheduled-last-result"
log="$state_dir/scheduled.log"
lock="$state_dir/scheduled.lock"
deployed_tree="$state_dir/deployed-git-tree"
marker="EASYNETTOPO_SCHEDULE:$repo_dir"

usage() {
    cat <<'USAGE'
用法：sudo bash deploy.sh schedule start <检查频率>
      sudo bash deploy.sh schedule status
      sudo bash deploy.sh schedule logs
      sudo bash deploy.sh schedule stop
例：start 15m、start 2h、start 1d 或 start 15（分钟）
频率支持 1～10080 分钟；仅有新 Git 文件树或尚无成功部署记录时构建。
USAGE
}

[[ "$repo_dir" != *$'\n'* && "$repo_dir" != *$'\r'* && "$repo_dir" != *"'"* && "$repo_dir" != *%* ]] ||
    fail '定时任务的项目路径不能包含单引号、百分号或换行。'
[[ ! -L "$repo_dir/data" && ! -L "$state_dir" ]] || fail '定时任务不支持 data/ 或 data/run/ 符号链接。'
mkdir -p -- "$state_dir"

read_crontab() { crontab -l 2>/dev/null || :; }
# Read and change only the exact managed line, never other cron entries.
filter_managed() {
    EASYNETTOPO_CRON_MARKER="$marker" awk -v mode="$1" '
        BEGIN { suffix = " # " ENVIRON["EASYNETTOPO_CRON_MARKER"] }
        {
            is_ours = length($0) >= length(suffix) &&
                substr($0, length($0) - length(suffix) + 1) == suffix
            if (is_ours) found = 1
            else if (mode == "without") print
        }
        END { if (mode == "installed") exit !found }
    '
}
installed() { read_crontab | filter_managed installed; }
without_ours() { read_crontab | filter_managed without; }
require_cron() { command -v crontab >/dev/null 2>&1 || fail '系统未安装 crontab；请先在 FnOS 启用 cron。'; }
require_root() { (( EUID == 0 )) || fail '定时管理需通过 sudo 执行，确保 cron 可调用 Docker。'; }

parse_interval() {
    local input=$1 digits factor
    case "$input" in
        *m) digits=${input%m}; factor=1 ;;
        *h) digits=${input%h}; factor=60 ;;
        *d) digits=${input%d}; factor=1440 ;;
        *) digits=$input; factor=1 ;;
    esac
    [[ "$digits" =~ ^[0-9]{1,5}$ ]] || fail '检查频率请输入分钟数，或使用 15m / 2h / 1d。'
    digits=$((10#$digits))
    minutes=$((digits * factor))
    (( minutes >= 1 && minutes <= 10080 )) || fail '检查频率必须在 1 分钟至 7 天之间。'
}

read_config() {
    [[ -f "$config" && ! -L "$config" ]] || return 1
    local extra
    { IFS= read -r interval || return 1
      IFS= read -r git_uid || return 1
      if IFS= read -r extra; then return 1; fi
    } < "$config"
    [[ "$interval" =~ ^[0-9]{1,5}$ && "$git_uid" =~ ^[0-9]{1,10}$ ]] || return 1
    (( interval >= 1 && interval <= 10080 )) || return 1
}

atomic_line() {
    local dest=$1 value=$2 temp="$1.tmp.$$"
    printf '%s\n' "$value" > "$temp"
    chmod 600 "$temp"
    mv -f -- "$temp" "$dest"
}

# Display the live .env branch rather than persisting stale branch selection.
# .env is read as data, never executed as shell code.
effective_branch() {
    local line value selected=''
    if [[ -f "$repo_dir/.env" ]]; then
        while IFS= read -r line || [[ -n "$line" ]]; do
            line=${line%$'\r'}
            if [[ "$line" =~ ^[[:space:]]*GIT_BRANCH[[:space:]]*=(.*)$ ]]; then
                value=${BASH_REMATCH[1]}
                value=${value#"${value%%[![:space:]]*}"}
                value=${value%"${value##*[![:space:]]}"}
                if [[ ${#value} -ge 2 && ( "$value" == \"*\" || "$value" == \'*\' ) ]]; then
                    value=${value:1:${#value}-2}
                fi
                selected=$value
            fi
        done < "$repo_dir/.env"
    fi
    if [[ -n "$selected" ]]; then printf '%s\n' "$selected"
    elif [[ -n "${git_uid:-}" ]]; then git_as_owner symbolic-ref --quiet --short HEAD || printf '(未检出分支)\n'
    else git -C "$repo_dir" symbolic-ref --quiet --short HEAD || printf '(未检出分支)\n'; fi
}

# Operate in the same Git security context as the original checkout owner.
git_as_owner() {
    if (( EUID == 0 )) && [[ "$git_uid" != 0 ]]; then
        sudo -n -H -u "#$git_uid" env GIT_TERMINAL_PROMPT=0 \
            GIT_SSH_COMMAND='ssh -o BatchMode=yes' \
            git -C "$repo_dir" "$@"
    else
        GIT_TERMINAL_PROMPT=0 GIT_SSH_COMMAND='ssh -o BatchMode=yes' git -C "$repo_dir" "$@"
    fi
}

run_check() {
    read_config || fail '定时配置损坏；请重新执行 schedule start。'
    installed || return 0
    command -v flock >/dev/null 2>&1 || fail '缺少 flock。'
    exec 8>"$lock"
    flock -n 8 || exit 0
    local now previous result before after rc
    now=$(date +%s)
    previous=$(cat "$last_check" 2>/dev/null || printf '0')
    [[ "$previous" =~ ^[0-9]{1,12}$ ]] || previous=0
    (( now >= previous + interval * 60 )) || return 0
    atomic_line "$last_check" "$now"
    printf '[%s] 检查 Git 目标分支（间隔 %s 分钟）。\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$interval"
    before=$(cat "$deployed_tree" 2>/dev/null || :)
    if EASYNETTOPO_GIT_UID="$git_uid" bash "$repo_dir/deploy.sh" check; then
        after=$(cat "$deployed_tree" 2>/dev/null || :)
        if [[ "$before" == "$after" ]]; then
            result='无 Git 文件树变化，跳过部署'
        else
            result='发现源码变化，部署成功'
        fi
        atomic_line "$last_result" "$result"
        printf '[%s] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$result"
    else
        rc=$?
        if (( rc == 75 )); then
            # Manual deployment holds the main lock: do not consume the interval.
            atomic_line "$last_check" "$previous"
            result='已有部署正在执行，本轮跳过，下一分钟重试'
            atomic_line "$last_result" "$result"
            printf '[%s] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$result"
            return 0
        fi
        result="检查/部署失败（退出码 $rc）；等待下一次检查重试"
        atomic_line "$last_result" "$result"
        printf '[%s] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$result"
        return "$rc"
    fi
}

case "${1:-}" in
    start)
        (( $# == 2 )) || { usage; exit 2; }
        require_root; require_cron
        command -v flock >/dev/null 2>&1 || fail '缺少 flock。'
        parse_interval "$2"
        # SUDO_UID is numeric even when the original user's login name is numeric.
        git_uid=${SUDO_UID:-$(id -u)}
        [[ "$git_uid" =~ ^[0-9]{1,10}$ ]] || fail '无法识别原始 Git 用户 UID。'
        command -v sudo >/dev/null 2>&1 || fail '需要 sudo 以仓库所属用户拉取代码。'
        git_as_owner rev-parse --is-inside-work-tree >/dev/null || fail '原始 Git 用户无法访问仓库。'
        [[ "$(git_as_owner rev-parse --show-toplevel)" == "$repo_dir" ]] || fail '脚本必须位于独立 Git 仓库根目录。'
        # Avoid losing unrelated cron jobs; reject malformed crontab before writing.
        previous_table=$(without_ours)
        cron_line="* * * * * /bin/bash '$repo_dir/deploy.sh' schedule run >> '$log' 2>&1 # $marker"
        tmp_config="$config.tmp.$$"
        printf '%s\n%s\n' "$minutes" "$git_uid" > "$tmp_config"
        chmod 600 "$tmp_config"
        if ! {
            [[ -z "$previous_table" ]] || printf '%s\n' "$previous_table"
            printf '%s\n' "$cron_line"
        } | crontab -; then
            rm -f -- "$tmp_config"
            fail '写入 crontab 失败。'
        fi
        mv -f -- "$tmp_config" "$config"
        touch "$log"; chmod 600 "$log"
        atomic_line "$last_check" "$(date +%s)"
        atomic_line "$last_result" '已启用；等待首次周期检查'
        printf '已开启：每 %s 分钟检查一次。\n' "$minutes"
        printf '运行账户：root（Git 操作使用 UID %s）。\n' "$git_uid"
        printf '目标分支：%s（来自 .env / 当前分支）\n' "$(effective_branch)"
        printf '日志：%s\n' "$log"
        printf '注意：FnOS 上 cron/crond 服务需要保持运行。\n'
        ;;
    status)
        (( $# == 1 )) || { usage; exit 2; }
        require_cron
        if installed; then printf '定时状态：已启用（已安装 cron 任务）\n'
        else printf '定时状态：已关闭（没有受管 cron 任务）\n'; fi
        if command -v pgrep >/dev/null 2>&1 &&
           { pgrep -x cron >/dev/null 2>&1 || pgrep -x crond >/dev/null 2>&1; }; then
            printf 'cron 服务：检测到运行进程\n'
        else
            printf 'cron 服务：未检测到 cron/crond，需确认 FnOS 定时服务状态\n'
        fi
        if read_config; then
            printf '检查频率：%s 分钟\nGit 用户 UID：%s\n' "$interval" "$git_uid"
            printf '目标分支：%s（来自 .env / 当前分支）\n' "$(effective_branch)"
        elif [[ -e "$config" ]]; then
            printf '警告：定时配置损坏，请重新执行 schedule start。\n'
        fi
        if [[ -f "$last_check" ]]; then
            stamp=$(cat "$last_check" || :)
            if [[ "$stamp" =~ ^[0-9]{1,12}$ ]]; then
                printf '最近检查时间：%s\n' "$(date -d "@$stamp" '+%Y-%m-%d %H:%M:%S' 2>/dev/null || echo "$stamp")"
                if installed && read_config; then
                    next_stamp=$((stamp + interval * 60))
                    printf '下次预计检查：%s\n' "$(date -d "@$next_stamp" '+%Y-%m-%d %H:%M:%S' 2>/dev/null || echo "$next_stamp")"
                fi
            fi
        fi
        [[ ! -f "$last_result" ]] || printf '最近结果：%s\n' "$(cat "$last_result")"
        [[ ! -f "$deployed_tree" ]] || printf '最近成功部署 Git 树：%s\n' "$(cat "$deployed_tree")"
        printf '日志：%s\n' "$log"
        ;;
    stop)
        (( $# == 1 )) || { usage; exit 2; }
        require_root; require_cron
        if installed; then
            previous_table=$(without_ours)
            if [[ -n "$previous_table" ]]; then printf '%s\n' "$previous_table" | crontab -
            else printf '' | crontab -; fi
        fi
        printf '定时检查已停止（正在执行的部署不会被强制中断）。\n'
        ;;
    logs)
        (( $# == 1 )) || { usage; exit 2; }
        if [[ -f "$log" ]]; then tail -n 80 "$log"
        else printf '暂无定时部署日志。\n'; fi
        ;;
    run)
        (( $# == 1 )) || { usage; exit 2; }
        require_root; require_cron
        run_check
        ;;
    help|-h|--help) usage ;;
    *) usage >&2; exit 2 ;;
esac
