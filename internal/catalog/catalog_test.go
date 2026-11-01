package catalog

import (
 "context"
 "strings"
 "testing"
)

func TestGitURLAllowlistAndBranch(t *testing.T) {
 t.Setenv("EASYNETTOPO_GIT_ALLOWED_HOSTS","github.com")
 for _,url:=range []string{"git@github.com:CiBeDerORG/deploy.git","https://github.com/CiBeDerORG/deploy.git","https://github.com/CiBeDerORG/deploy"}{
  if err:=AllowedRemote(url);err!=nil{t.Fatalf("有效仓库被拒绝 %s: %v",url,err)}
 }
 for _,url:=range []string{
  "file:///etc/passwd","ssh://git@github.com/CiBeDerORG/deploy.git",
  "https://evil.example/CiBeDerORG/deploy.git",
  "https://token@github.com/CiBeDerORG/deploy.git",
  "https://github.com:443/CiBeDerORG/deploy.git",
  "https://github.com/../deploy.git",
  "git@127.0.0.1:CiBeDerORG/deploy.git",
  "git@github.com:CiBeDerORG/../deploy.git",
  "https://github.com/CiBeDerORG/deploy.git?x=1",
 }{
  if AllowedRemote(url)==nil{t.Errorf("危险 Git 地址被允许: %s",url)}
 }
 for _,v:=range []string{"--upload-pack=evil","../main","main..old","main//test","release.lock"}{
  if validateBranch(v)==nil{t.Errorf("危险分支被允许 %q",v)}
 }
}
func TestReadOnlyManifestValidation(t *testing.T) {
 raw:=[]byte(`schema_version: 1
id: sub2api
name: Sub2API
summary: 服务与依赖
category: developer
runtime:
  kind: docker_compose
  compose_project: sub2api
  compose_file: deploy/docker-compose.yaml
package:
  files:
    - source: sub2api/deploy/docker-compose.yaml
      destination: sub2api/deploy/docker-compose.yaml
verification:
  required_services: [sub2api, postgres, redis]
  state: healthy
data:
  default_retain: true
`)
 objects:=map[string]gitObject{"sub2api/deploy/docker-compose.yaml":{Mode:"100644",Object:strings.Repeat("a",40)}}
 manifest,sha,err:=previewManifest("sub2api",raw,objects)
 if err!=nil||manifest.Name!="Sub2API"||len(sha)!=64{t.Fatalf("正常应用预览失败: %v",err)}
 if _,_,err:=previewManifest("other",raw,objects);err==nil{t.Fatal("允许不一致的目录 ID")}
 broken:=strings.Replace(string(raw),"sub2api/deploy/docker-compose.yaml","../etc/passwd",1)
 if _,_,err:=previewManifest("sub2api",[]byte(broken),objects);err==nil{t.Fatal("接受路径穿越")}
 symlink:=map[string]gitObject{"sub2api/deploy/docker-compose.yaml":{Mode:"120000",Object:strings.Repeat("a",40)}}
 if _,_,err:=previewManifest("sub2api",raw,symlink);err==nil{t.Fatal("接受符号链接资源")}
}
func TestGitExecVersionIsReadOnly(t *testing.T){
 output,err:=gitCommand(context.Background(),1024,"--version")
 if err!=nil{t.Skip("测试环境未安装 Git: "+err.Error())}
 if !strings.Contains(string(output),"git version"){t.Fatal("Git CLI 不可用")}
}
