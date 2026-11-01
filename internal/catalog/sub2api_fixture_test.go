package catalog

import (
 "os"
 "testing"

 "gopkg.in/yaml.v3"
)

func TestPublishedSub2APIManifestIsPreviewable(t *testing.T) {
 raw,err:=os.ReadFile("testdata/sub2api.app.yaml")
 if err!=nil{t.Fatal(err)}
 var fixture appManifest
 if err:=yaml.Unmarshal(raw,&fixture);err!=nil{t.Fatal(err)}
 if fixture.ID!="sub2api"||len(fixture.Package.Files)!=5{t.Fatal("Sub2API 样板预期的包文件数量变更")}
 objects:=make(map[string]gitObject)
 for _,f:=range fixture.Package.Files{objects[f.Source]=gitObject{Mode:"100644",Object:"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}
 m,digest,err:=previewManifest("sub2api",raw,objects)
 if err!=nil{t.Fatalf("发布模板无法显示在商店: %v",err)}
 if m.Name!="Sub2API"||m.Runtime.Project!="sub2api"||digest==""{t.Fatal("发布模板字段识别异常")}
 if len(m.Verification.Services)!=3||m.Verification.State!="healthy"{t.Fatal("未识别三个必须健康的容器")}
}
