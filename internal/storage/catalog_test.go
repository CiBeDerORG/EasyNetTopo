package storage

import (
 "context"
 "errors"
 "path/filepath"
 "testing"
)

func TestCatalogSnapshotAtomicity(t *testing.T){
 ctx:=context.Background()
 s,err:=Open(filepath.Join(t.TempDir(),"catalog.db"));if err!=nil{t.Fatal(err)}
 defer s.Close()
 if err:=s.EnsureCatalogSchema(ctx);err!=nil{t.Fatal(err)}
 source,err:=s.CreateCatalogSource(ctx,"Deploy","git@github.com:CiBeDerORG/deploy.git","main")
 if err!=nil{t.Fatal(err)}
 entry:=CatalogApp{SourceID:source.ID,AppID:"sub2api",Name:"Sub2API",Summary:"服务",Category:"developer",Revision:"a",ManifestSHA256:"digest"}
 if err:=s.SaveCatalogSnapshot(ctx,source.ID,"a",[]CatalogApp{entry});err!=nil{t.Fatal(err)}
 conflict:=entry
 conflict.SourceID="different"
 conflict.Revision="b"
 if s.SaveCatalogSnapshot(ctx,source.ID,"b",[]CatalogApp{conflict})==nil{t.Fatal("不一致快照被接受")}
 actual,err:=s.CatalogApps(ctx);if err!=nil{t.Fatal(err)}
 if len(actual)!=1||actual[0].Revision!="a"{t.Fatalf("失败刷新污染成功快照: %+v",actual)}
 if err:=s.SetCatalogSyncError(ctx,source.ID);err!=nil{t.Fatal(err)}
 sources,err:=s.CatalogSources(ctx);if err!=nil{t.Fatal(err)}
 if len(sources)!=1||sources[0].CurrentRevision!="a"||sources[0].LastError==""{t.Fatalf("错误刷新覆盖既有信息: %+v",sources)}
 if err:=s.SaveCatalogSnapshot(ctx,source.ID,"c",nil);err!=nil{t.Fatal(err)}
 actual,err=s.CatalogApps(ctx);if err!=nil||len(actual)!=0{t.Fatal("空快照未正确提交",err)}
 source,err=s.CatalogSourceByID(ctx,source.ID);if err!=nil||source.CurrentRevision!="c"||source.LastError!=""{t.Fatal("成功刷新没有清除失败状态")}
 _,err=s.CatalogSourceByID(ctx,"missing")
 if !errors.Is(err,ErrSourceNotFound){t.Fatal("未知来源错误类型不正确",err)}
}
