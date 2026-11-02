package catalog

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var ownerRepo = regexp.MustCompile("^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+(?:[.]git)?$")
var label = regexp.MustCompile("^[A-Za-z0-9][A-Za-z0-9._-]*$")
var dnsGitHost = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?$`)

// gitHostKey 限制主机名及显式端口，阻止隐式用户信息与危险 IP。
// 内网 IPv4/IPv6 必须由运行管理员精确加入白名单；禁止回环、链路本地、未指定及组播地址。
func gitHostKey(host, port string) (string, error) {
	if host == "" || strings.ToLower(host) != host || strings.ContainsAny(host, "@/%\\") {
		return "", errors.New("Git 主机名格式不合法")
	}
	if port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 || strconv.Itoa(n) != port {
			return "", errors.New("Git 端口必须为 1 至 65535 的十进制数")
		}
	}
	var key string
	if ip := net.ParseIP(host); ip != nil {
		if ip.IsLoopback() || ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return "", errors.New("不允许访问回环、链路本地或特殊 IP 地址")
		}
		if strings.Contains(host, ":") {
			key = "[" + strings.ToLower(ip.String()) + "]"
		} else {
			key = ip.String()
		}
	} else {
		if len(host) > 253 || !dnsGitHost.MatchString(host) || strings.Contains(host, "..") || !strings.Contains(host, ".") {
			return "", errors.New("Git 域名格式不合法")
		}
		for _, part := range strings.Split(host, ".") {
			if part == "" || len(part) > 63 || strings.HasPrefix(part, "-") || strings.HasSuffix(part, "-") {
				return "", errors.New("Git 域名标签格式不合法")
			}
		}
		key = host
	}
	if port != "" {
		key += ":" + port
	}
	return key, nil
}

func allowedGitHosts() (map[string]bool, bool, error) {
	raw := strings.TrimSpace(os.Getenv("EASYNETTOPO_GIT_ALLOWED_HOSTS"))
	if raw == "" {
		return map[string]bool{"github.com": true}, false, nil
	}
	hosts := make(map[string]bool)
	for _, value := range strings.Split(raw, ",") {
		entry := strings.TrimSpace(value)
		if entry == "" {
			return nil, false, errors.New("Git 域名白名单不允许空项")
		}
		parsed, err := url.Parse("https://" + entry)
		if err != nil || parsed.User != nil || parsed.Host != entry || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
			return nil, false, errors.New("Git 白名单必须为精确 host 或 host:port")
		}
		key, err := gitHostKey(parsed.Hostname(), parsed.Port())
		if err != nil || key != entry {
			return nil, false, errors.New("Git 白名单包含不安全的地址或端口")
		}
		hosts[key] = true
	}
	return hosts, true, nil
}

// AllowedRemote 只允许可审计的 Git SSH/HTTPS/HTTP 来源。
// HTTP 是纯文本协议：必须显式启用且管理员明确允许目标 host[:port]，不支持 URL 凭据。
func AllowedRemote(raw string) error {
	hosts, explicit, err := allowedGitHosts()
	if err != nil {
		return err
	}
	var key, repoPath string
	if strings.HasPrefix(raw, "git@") {
		parts := strings.SplitN(strings.TrimPrefix(raw, "git@"), ":", 2)
		if len(parts) != 2 {
			return errors.New("SSH Git URL 格式不合法")
		}
		key, err = gitHostKey(parts[0], "")
		if err != nil {
			return err
		}
		repoPath = parts[1]
	} else {
		parsed, parseErr := url.Parse(raw)
		if parseErr != nil || parsed == nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.User != nil ||
			parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Opaque != "" || parsed.RawPath != "" ||
			strings.Contains(raw, "%") || strings.HasSuffix(parsed.Host, ":") {
			return errors.New("仅支持无凭据的 HTTPS、受控 HTTP 或 git@host:owner/repo.git")
		}
		key, err = gitHostKey(parsed.Hostname(), parsed.Port())
		if err != nil {
			return err
		}
		if parsed.Scheme == "http" {
			if !explicit || os.Getenv("EASYNETTOPO_GIT_ALLOW_HTTP") != "true" {
				return errors.New("HTTP Git 来源必须由服务端显式启用并将 host[:port] 加入白名单")
			}
		}
		repoPath = strings.TrimPrefix(parsed.Path, "/")
	}
	if !hosts[key] || !ownerRepo.MatchString(repoPath) {
		return fmt.Errorf("Git 地址不在已批准的主机或仓库路径范围内")
	}
	for _, segment := range strings.Split(repoPath, "/") {
		if segment == "." || segment == ".." || strings.HasPrefix(segment, ".") || strings.Contains(segment, "..") {
			return errors.New("Git 仓库路径包含不安全的段")
		}
	}
	return nil
}

func validateBranch(value string) error {
	if !refName.MatchString(value) || strings.Contains(value, "..") || strings.Contains(value, "//") || strings.HasSuffix(value, "/") || strings.HasSuffix(value, ".lock") || strings.Contains(value, "@{") {
		return errors.New("分支名格式不合法")
	}
	return nil
}
