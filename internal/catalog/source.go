package catalog

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
)

var ownerRepo = regexp.MustCompile("^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+(?:[.]git)?$")
var label = regexp.MustCompile("^[A-Za-z0-9][A-Za-z0-9._-]*$")

// AllowedRemote 从部署人员预先批准的 Git 域名读取，默认只开放 github.com。
func AllowedRemote(raw string) error {
	hosts := map[string]bool{"github.com": true}
	if env := os.Getenv("EASYNETTOPO_GIT_ALLOWED_HOSTS"); env != "" {
		hosts = map[string]bool{}
		for _, part := range strings.Split(env, ",") {
			host := strings.ToLower(strings.TrimSpace(part))
			if host == "" || strings.ContainsAny(host, "/:@ \\") {
				return errors.New("Git 域名白名单格式错误")
			}
			hosts[host] = true
		}
	}
	var host, path string
	if strings.HasPrefix(raw, "git@") {
		parts := strings.SplitN(strings.TrimPrefix(raw, "git@"), ":", 2)
		if len(parts) != 2 {
			return errors.New("无效 SSH Git URL")
		}
		host = parts[0]
		path = parts[1]
	} else {
		parsed, err := url.Parse(raw)
		if err != nil || parsed.Scheme != "https" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Port() != "" || parsed.Opaque != "" {
			return errors.New("仅允许不含凭据的 HTTPS Git URL 或 git@host:owner/repo.git")
		}
		host = parsed.Hostname()
		path = strings.TrimPrefix(parsed.Path, "/")
	}
	if !hosts[strings.ToLower(host)] || host != strings.ToLower(host) || !ownerRepo.MatchString(path) {
		return fmt.Errorf("Git 地址不在允许的域名或仓库路径范围内")
	}
	for _, segment := range strings.Split(path, "/") {
		if segment == "." || segment == ".." || strings.HasPrefix(segment, ".") || strings.Contains(segment, "..") {
			return errors.New("不支持不安全的仓库路径")
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
