package catalog

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var commitID = regexp.MustCompile("^[a-f0-9]{40}$")
var refName = regexp.MustCompile("^[A-Za-z0-9][A-Za-z0-9_./-]{0,99}$")

type gitObject struct {
	Mode   string
	Object string
}

// gitRepository 仅读取 Git 对象，不 checkout 外部仓库代码。
type gitRepository struct {
	path     string
	revision string
	objects  map[string]gitObject
}

type limitedOutput struct {
	bytes.Buffer
	limit int
}

func (b *limitedOutput) Write(p []byte) (int, error) {
	if b.Len()+len(p) > b.limit {
		return 0, errors.New("Git 输出超过上限")
	}
	return b.Buffer.Write(p)
}
func gitCommand(ctx context.Context, limit int, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_COUNT=5", "GIT_CONFIG_KEY_0=core.hooksPath", "GIT_CONFIG_VALUE_0=/dev/null",
		"GIT_CONFIG_KEY_1=protocol.file.allow", "GIT_CONFIG_VALUE_1=never",
		"GIT_CONFIG_KEY_2=http.followRedirects", "GIT_CONFIG_VALUE_2=false",
		"GIT_CONFIG_KEY_3=credential.helper", "GIT_CONFIG_VALUE_3=",
		"GIT_CONFIG_KEY_4=http.cookieFile", "GIT_CONFIG_VALUE_4=/dev/null",
		"GIT_SSL_NO_VERIFY=0",
		"GIT_SSH_COMMAND=ssh -F /dev/null -o BatchMode=yes -o StrictHostKeyChecking=yes -o ClearAllForwardings=yes",
	)
	out := &limitedOutput{limit: limit}
	errOut := &limitedOutput{limit: 4096}
	cmd.Stdout = out
	cmd.Stderr = errOut
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("Git 操作失败: %w", err)
	}
	return out.Bytes(), nil
}
func refreshGit(ctx context.Context, cache, remote, branch string) (*gitRepository, error) {
	if !refName.MatchString(branch) || strings.Contains(branch, "..") || strings.Contains(branch, "//") || strings.HasSuffix(branch, "/") || strings.HasSuffix(branch, ".lock") {
		return nil, errors.New("Git 分支名不合法")
	}
	if err := os.MkdirAll(filepath.Dir(cache), 0700); err != nil {
		return nil, err
	}
	if _, err := os.Stat(cache); errors.Is(err, os.ErrNotExist) {
		if _, err = gitCommand(ctx, 4096, "clone", "--bare", "--single-branch", "--depth=1", "--branch", branch, "--", remote, cache); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	} else {
		if _, err = gitCommand(ctx, 4096, "--git-dir="+cache, "fetch", "--no-tags", "--depth=1", remote, "+refs/heads/"+branch+":refs/heads/"+branch); err != nil {
			return nil, err
		}
	}
	ref, err := gitCommand(ctx, 256, "--git-dir="+cache, "rev-parse", "refs/heads/"+branch)
	if err != nil {
		return nil, err
	}
	revision := strings.TrimSpace(string(ref))
	if !commitID.MatchString(revision) {
		return nil, errors.New("Git Commit 不是预期 SHA-1")
	}
	tree, err := gitCommand(ctx, 4*1024*1024, "--git-dir="+cache, "ls-tree", "-r", "-z", "--full-tree", revision)
	if err != nil {
		return nil, err
	}
	objects := make(map[string]gitObject)
	for _, row := range bytes.Split(tree, []byte{0}) {
		if len(row) == 0 {
			continue
		}
		if len(objects) >= 10000 {
			return nil, errors.New("Git 仓库文件数过多")
		}
		split := bytes.SplitN(row, []byte{'\t'}, 2)
		if len(split) != 2 {
			return nil, errors.New("Git 树对象格式错误")
		}
		fields := strings.Fields(string(split[0]))
		if len(fields) != 3 || fields[1] != "blob" {
			continue
		}
		objects[string(split[1])] = gitObject{Mode: fields[0], Object: fields[2]}
	}
	return &gitRepository{path: cache, revision: revision, objects: objects}, nil
}
func (r *gitRepository) contents(ctx context.Context, path string) ([]byte, error) {
	entry, ok := r.objects[path]
	if !ok || !(entry.Mode == "100644" || entry.Mode == "100755") {
		return nil, errors.New("文件不存在或不是普通文件")
	}
	if !commitID.MatchString(entry.Object) {
		return nil, errors.New("Git 对象摘要不合法")
	}
	return gitCommand(ctx, 128*1024, "--git-dir="+r.path, "cat-file", "blob", entry.Object)
}
func (r *gitRepository) pin(ctx context.Context) error {
	_, err := gitCommand(ctx, 4096, "--git-dir="+r.path, "update-ref", "refs/easynettopo/catalog/"+r.revision, r.revision)
	return err
}
