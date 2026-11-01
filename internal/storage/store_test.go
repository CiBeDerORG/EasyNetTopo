package storage

import (
 "context"
 "errors"
 "path/filepath"
 "testing"
)
func TestAdminSessionAndControllerIdentityPersist(t *testing.T){
 ctx:=context.Background();path:=filepath.Join(t.TempDir(),"state","app.db")
 s,err:=Open(path);if err!=nil{t.Fatal(err)}
 id:=s.ControllerID()
 initialized,err:=s.Initialized(ctx);if err!=nil||initialized{t.Fatal("数据库初始化状态错误",err)}
 if err:=s.CreateAdmin(ctx,"admin","correct-horse-battery");err!=nil{t.Fatal(err)}
 if err:=s.CreateAdmin(ctx,"admin","change-without-auth");!errors.Is(err,ErrInitialized){t.Fatal("允许覆盖管理员",err)}
 if s.Verify(ctx,"admin","wrong") == nil {t.Fatal("错误密码通过")}
 if err:=s.Verify(ctx,"admin","correct-horse-battery");err!=nil{t.Fatal(err)}
 token,_,err:=s.NewSession(ctx);if err!=nil||!s.HasSession(ctx,token){t.Fatal("会话创建失败",err)}
 if err:=s.Close();err!=nil{t.Fatal(err)}
 s,err=Open(path);if err!=nil{t.Fatal(err)};defer s.Close()
 if s.ControllerID()!=id||!s.HasSession(ctx,token){t.Fatal("身份或会话未持久化")}
 if err:=s.Revoke(ctx,token);err!=nil{t.Fatal(err)}
 if s.HasSession(ctx,token){t.Fatal("注销失败")}
}
func TestBootstrapGuards(t *testing.T){
 if _,err:=Open("relative.db");err==nil{t.Fatal("允许相对路径")}
 s,err:=Open(filepath.Join(t.TempDir(),"app.db"));if err!=nil{t.Fatal(err)};defer s.Close()
 for _,v:=range []struct{u,p string}{{"root","correct-horse-battery"},{"admin","short"},{"admin",string(make([]byte,73))}}{
  if s.CreateAdmin(context.Background(),v.u,v.p)==nil{t.Fatal("允许错误的初始化参数")}
 }
}
