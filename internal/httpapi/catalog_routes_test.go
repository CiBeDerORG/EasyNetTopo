package httpapi

import (
 "context"
 "encoding/json"
 "net/http"
 "net/http/httptest"
 "path/filepath"
 "strings"
 "testing"

 "github.com/CiBeDerORG/EasyNetTopo/internal/catalog"
 "github.com/CiBeDerORG/EasyNetTopo/internal/storage"
)

func TestCatalogRoutesAuthorizationAndSourceValidation(t *testing.T){
 t.Setenv("EASYNETTOPO_GIT_ALLOWED_HOSTS","github.com")
 db,err:=storage.Open(filepath.Join(t.TempDir(),"app.db"));if err!=nil{t.Fatal(err)}
 defer db.Close()
 service,err:=catalog.NewService(context.Background(),db);if err!=nil{t.Fatal(err)}
 token,_,err:=db.NewSession(context.Background());if err!=nil{t.Fatal(err)}
 handler:=NewHandler(t.TempDir(),db,service)
 call:=func(method,path,body string,authorized,mutation bool,origin string)*httptest.ResponseRecorder{
  t.Helper()
  req:=httptest.NewRequest(method,path,strings.NewReader(body))
  if authorized{req.AddCookie(&http.Cookie{Name:authCookie,Value:token})}
  if mutation{req.Header.Set("X-ENT-Request","1")}
  if body!=""{req.Header.Set("Content-Type","application/json")}
  if origin!=""{req.Header.Set("Origin",origin)}
  w:=httptest.NewRecorder();handler.ServeHTTP(w,req);return w
 }
 if got:=call("GET","/api/v1/catalog/sources","",false,false,"");got.Code!=401{t.Fatal("未登录可读应用源",got.Code)}
 body:=`{"name":"Deploy","url":"git@github.com:CiBeDerORG/deploy.git","branch":"main"}`
 if got:=call("POST","/api/v1/catalog/sources",body,true,false,"");got.Code!=403{t.Fatal("无 CSRF 标记可以创建来源",got.Code)}
 if got:=call("POST","/api/v1/catalog/sources",body,true,true,"https://malicious.test");got.Code!=403{t.Fatal("外部 Origin 被允许",got.Code)}
 if got:=call("POST","/api/v1/catalog/sources",strings.ReplaceAll(body,"git@github.com:CiBeDerORG/deploy.git","file:///etc/hosts"),true,true,"");got.Code!=400{t.Fatal("不安全 Git URL 未拒绝",got.Code)}
 created:=call("POST","/api/v1/catalog/sources",body,true,true,"")
 if created.Code!=201{t.Fatalf("合法来源创建失败: %d %s",created.Code,created.Body.String())}
 var s storage.CatalogSource
 if err:=json.Unmarshal(created.Body.Bytes(),&s);err!=nil{t.Fatal(err)}
 if len(s.ID)!=36{t.Fatal("无稳定 UUID")}
 if got:=call("POST","/api/v1/catalog/sources",body,true,true,"");got.Code!=400{t.Fatal("重复来源未拒绝",got.Code)}
 list:=call("GET","/api/v1/catalog/sources","",true,false,"")
 if list.Code!=200||!strings.Contains(list.Body.String(),s.ID){t.Fatal("来源读取失败")}
 bad:=call("POST","/api/v1/catalog/sources/00000000-0000-4000-8000-000000000000/refresh","",true,true,"")
 if bad.Code!=404{t.Fatal("未知来源没有返回 404",bad.Code)}
 apps:=call("GET","/api/v1/catalog/apps","",true,false,"")
 if apps.Code!=200||!strings.Contains(apps.Body.String(),`"install_enabled":false`){t.Fatal("应用安装安全开关异常")}
}
