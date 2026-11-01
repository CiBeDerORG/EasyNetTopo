package catalog

import (
 "bytes"
 "crypto/sha256"
 "encoding/hex"
 "errors"
 "path"
 "regexp"
 "strings"

 "gopkg.in/yaml.v3"
)

var appID=regexp.MustCompile("^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$")
var safeFile=regexp.MustCompile("^[A-Za-z0-9_.-]+(?:/[A-Za-z0-9_.-]+)*$")

type appManifest struct{
 Version int `yaml:"schema_version"`
 ID string `yaml:"id"`
 Name string `yaml:"name"`
 Summary string `yaml:"summary"`
 Category string `yaml:"category"`
 Runtime struct {
  Kind string `yaml:"kind"`
  Project string `yaml:"compose_project"`
  File string `yaml:"compose_file"`
 } `yaml:"runtime"`
 Package struct{ Files []struct {
  Source string `yaml:"source"`
  Destination string `yaml:"destination"`
 } `yaml:"files"` } `yaml:"package"`
 Verification struct{
  Services []string `yaml:"required_services"`
  State string `yaml:"state"`
 } `yaml:"verification"`
 Data struct{DefaultRetain bool `yaml:"default_retain"`} `yaml:"data"`
}

func safePath(p string)bool{
 if len(p)==0||len(p)>256||!safeFile.MatchString(p){return false}
 for _,part:=range strings.Split(p,"/"){
  if part=="."||part==".."||strings.HasPrefix(part,".")||part=="data"||part=="runtime"||part==".env"||part==".secrets"{return false}
 }
 return path.Clean(p)==p
}
// previewManifest 仅用于只读商店展示；不能作为安装执行的全部 Schema/语义验证。
func previewManifest(id string,raw []byte,objects map[string]gitObject)(appManifest,string,error){
 var m appManifest
 if len(raw)>128*1024{return m,"",errors.New("Manifest 超出大小限制")}
 decoder:=yaml.NewDecoder(bytes.NewReader(raw))
 if err:=decoder.Decode(&m);err!=nil{return m,"",err}
 if m.Version!=1||m.ID!=id||!appID.MatchString(id)||len(id)>64||m.Name==""||len(m.Name)>100||m.Summary==""||len(m.Summary)>500 {
  return m,"",errors.New("Manifest 基本字段不满足 v1 约束")
 }
 if m.Runtime.Kind!="docker_compose"||m.Runtime.Project!=id||!safePath(m.Runtime.File)||len(m.Package.Files)==0||len(m.Package.Files)>128||len(m.Verification.Services)==0||!m.Data.DefaultRetain {
  return m,"",errors.New("Manifest 运行与数据声明不符合预览要求")
 }
 if m.Verification.State!="healthy"&&m.Verification.State!="running"{return m,"",errors.New("未知健康状态")}
 allowedCategories:=map[string]bool{"infrastructure":true,"network":true,"developer":true,"media":true,"productivity":true,"other":true}
 if !allowedCategories[m.Category]{return m,"",errors.New("未知应用分类")}
 destinations:=map[string]bool{}
 for _,f:=range m.Package.Files{
  if !safePath(f.Source)||!safePath(f.Destination){return m,"",errors.New("部署包路径不安全")}
  if !(strings.HasPrefix(f.Source,id+"/")||strings.HasPrefix(f.Source,"lib/")){return m,"",errors.New("包包含仓库外应用文件")}
  if !(strings.HasPrefix(f.Destination,id+"/")||strings.HasPrefix(f.Destination,"deploy/lib/")){return m,"",errors.New("部署目标超出允许范围")}
  if destinations[strings.ToLower(f.Destination)]{return m,"",errors.New("重复的部署目标")}
  destinations[strings.ToLower(f.Destination)]=true
  object,ok:=objects[f.Source]
  if !ok||(object.Mode!="100644"&&object.Mode!="100755"){return m,"",errors.New("缺少普通文件依赖")}
 }
 if !destinations[id+"/"+m.Runtime.File]{return m,"",errors.New("Compose 文件未列入包")}
 digest:=sha256.Sum256(raw)
 return m,hex.EncodeToString(digest[:]),nil
}
