package catalog

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"io"
	"path/filepath"
	"regexp"
	"strings"
)

var iconPath = regexp.MustCompile(`^assets/[A-Za-z0-9_-]+\.(svg|png|webp)$`)

func readIcon(ctx context.Context, repository *gitRepository, app, icon string) ([]byte, string, error) {
	if !iconPath.MatchString(icon) {
		return nil, "", errors.New("应用图标路径不合法")
	}
	raw, err := repository.contents(ctx, app+"/"+icon)
	if err != nil || len(raw) == 0 || len(raw) > 128*1024 {
		return nil, "", errors.New("图标缺失或超过 128 KiB")
	}
	mime := ""
	switch filepath.Ext(icon) {
	case ".png":
		if !bytes.HasPrefix(raw, []byte{137, 80, 78, 71, 13, 10, 26, 10}) {
			return nil, "", errors.New("PNG 图标内容无效")
		}
		mime = "image/png"
	case ".webp":
		if len(raw) < 12 || string(raw[:4]) != "RIFF" || string(raw[8:12]) != "WEBP" {
			return nil, "", errors.New("WebP 图标内容无效")
		}
		mime = "image/webp"
	case ".svg":
		decoder := xml.NewDecoder(bytes.NewReader(raw))
		root := false
		for {
			token, e := decoder.Token()
			if e == io.EOF {
				break
			}
			if e != nil {
				return nil, "", errors.New("SVG 图标内容无效")
			}
			if _, ok := token.(xml.Directive); ok {
				return nil, "", errors.New("SVG 不允许外部文档声明")
			}
			if element, ok := token.(xml.StartElement); ok {
				if !root && element.Name.Local != "svg" {
					return nil, "", errors.New("图标根元素必须是 SVG")
				}
				root = true
				if element.Name.Local == "script" || element.Name.Local == "foreignObject" {
					return nil, "", errors.New("SVG 不允许脚本或内嵌页面")
				}
				for _, attr := range element.Attr {
					if strings.HasPrefix(strings.ToLower(attr.Name.Local), "on") || (attr.Name.Local == "href" && !strings.HasPrefix(attr.Value, "#")) {
						return nil, "", errors.New("SVG 不允许事件或外部资源")
					}
				}
			}
		}
		if !root {
			return nil, "", errors.New("SVG 图标为空")
		}
		mime = "image/svg+xml"
	}
	return raw, mime, nil
}

// Icon 只读取已固定的来源快照；不把 Git 凭据或第三方 URL 暴露给浏览器。
func (s *Service) Icon(ctx context.Context, source, app, revision string) ([]byte, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !bundleSourceID.MatchString(source) || !appID.MatchString(app) || !commitID.MatchString(revision) {
		return nil, "", errors.New("图标请求无效")
	}
	if _, err := s.store.CatalogSourceByID(ctx, source); err != nil {
		return nil, "", err
	}
	repo, err := pinnedBundleRepository(ctx, filepath.Join(s.cacheDir, source+".git"), revision)
	if err != nil {
		return nil, "", err
	}
	raw, err := repo.contents(ctx, app+"/app.yaml")
	if err != nil {
		return nil, "", err
	}
	m, _, err := previewManifest(app, raw, repo.objects)
	if err != nil {
		return nil, "", err
	}
	return readIcon(ctx, repo, app, m.Icon)
}
