package httpapi

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"regexp"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/CiBeDerORG/EasyNetTopo/internal/catalog"
	"github.com/CiBeDerORG/EasyNetTopo/internal/hosts"
	"github.com/CiBeDerORG/EasyNetTopo/internal/storage"
)

var reverseAgentID = regexp.MustCompile(`^[a-f0-9]{8}-[a-f0-9]{4}-4[a-f0-9]{3}-[89ab][a-f0-9]{3}-[a-f0-9]{12}package httpapi

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"regexp"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/CiBeDerORG/EasyNetTopo/internal/catalog"
	"github.com/CiBeDerORG/EasyNetTopo/internal/hosts"
	"github.com/CiBeDerORG/EasyNetTopo/internal/storage"
)

)

type reverseGuard struct {
	mu sync.Mutex
	seen map[string]int64
}
func newReverseGuard() *reverseGuard { return &reverseGuard{seen:make(map[string]int64)} }

// 请求签名覆盖时间、nonce、HTTP 方法、路径和完整正文哈希；不得把 Agent 自报的证书指纹视为认证。
func (g *reverseGuard) verify(w http.ResponseWriter,r *http.Request,body []byte) (string,string,bool) {
	token:=os.Getenv("EASYNETTOPO_REVERSE_BOOTSTRAP_TOKEN")
	given,ok:=strings.CutPrefix(r.Header.Get("Authorization"),"Bearer ")
	if len(token)<32 || !ok || subtle.ConstantTimeCompare([]byte(given),[]byte(token))!=1 {
		http.Error(w,"反向接入凭据无效",http.StatusUnauthorized);return "","",false
	}
	if r.TLS==nil && os.Getenv("EASYNETTOPO_REVERSE_BEHIND_TLS_PROXY")!="true" {
		http.Error(w,"反向接入必须使用 TLS 或显式配置受信任的 TLS 反向代理",http.StatusUpgradeRequired);return "","",false
	}
	agentID:=r.Header.Get("X-EHA-Agent-ID")
	if !reverseAgentID.MatchString(agentID) {http.Error(w,"Agent UUID 格式无效",http.StatusUnauthorized);return "","",false}
	timestamp,err:=strconv.ParseInt(r.Header.Get("X-EHA-Timestamp"),10,64)
	if err!=nil || math.Abs(float64(time.Now().Unix()-timestamp))>120 {
		http.Error(w,"请求时间无效",http.StatusUnauthorized);return "","",false
	}
	nonce:=r.Header.Get("X-EHA-Nonce")
	if len(nonce)!=32 {http.Error(w,"nonce 无效",http.StatusUnauthorized);return "","",false}
	if _,err=hex.DecodeString(nonce);err!=nil {http.Error(w,"nonce 格式无效",http.StatusUnauthorized);return "","",false}
	der,err:=base64.RawURLEncoding.DecodeString(r.Header.Get("X-EHA-Cert"))
	if err!=nil || len(der)>4096 || len(der)==0 {http.Error(w,"Agent 证书无效",http.StatusUnauthorized);return "","",false}
	cert,err:=x509.ParseCertificate(der)
	if err!=nil || cert.Subject.CommonName!="EasyHostAgent "+agentID || time.Now().Before(cert.NotBefore) || time.Now().After(cert.NotAfter) {
		http.Error(w,"Agent 身份不符",http.StatusUnauthorized);return "","",false
	}
	pub,ok:=cert.PublicKey.(*ecdsa.PublicKey)
	if !ok || pub.Curve.Params().Name!="P-256" {http.Error(w,"仅允许 ECDSA P-256 身份",http.StatusUnauthorized);return "","",false}
	rawKey,err:=x509.MarshalPKIXPublicKey(pub)
	if err!=nil {http.Error(w,"Agent 公钥无效",http.StatusUnauthorized);return "","",false}
	pin:=sha256.Sum256(rawKey)
	digest:=sha256.Sum256(body)
	canonical:=strconv.FormatInt(timestamp,10)+"\n"+nonce+"\n"+r.Method+"\n"+r.URL.EscapedPath()+"\n"+hex.EncodeToString(digest[:])
	hash:=sha256.Sum256([]byte(canonical))
	sig,err:=base64.RawURLEncoding.DecodeString(r.Header.Get("X-EHA-Signature"))
	if err!=nil || !ecdsa.VerifyASN1(pub,hash[:],sig) {
		http.Error(w,"Agent 签名验证失败",http.StatusUnauthorized);return "","",false
	}
	key:=agentID+":"+nonce
	g.mu.Lock()
	for k,expiry:=range g.seen {if expiry<time.Now().Unix(){delete(g.seen,k)}}
	if len(g.seen)>20000 {g.mu.Unlock();http.Error(w,"验证请求过多",http.StatusTooManyRequests);return "","",false}
	if _,exists:=g.seen[key];exists {g.mu.Unlock();http.Error(w,"重放请求",http.StatusConflict);return "","",false}
	g.seen[key]=time.Now().Add(3*time.Minute).Unix()
	g.mu.Unlock()
	return agentID,hex.EncodeToString(pin[:]),true
}

func reverseRoutes(mux *http.ServeMux,db *storage.Store,hostSvc *hosts.Service,catalogSvc *catalog.Service) {
	guard:=newReverseGuard()
	write:=func(w http.ResponseWriter,status int,value any) {
		w.Header().Set("Content-Type","application/json; charset=utf-8")
		w.Header().Set("Cache-Control","no-store")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(value)
	}
	admin:=func(w http.ResponseWriter,r *http.Request,mutation bool) bool {
		c,err:=r.Cookie(authCookie)
		if err!=nil || !db.HasSession(r.Context(),c.Value) {http.Error(w,"需要管理员登录",http.StatusUnauthorized);return false}
		if !mutation {return true}
		if r.Header.Get("X-ENT-Request")!="1" {http.Error(w,"缺少同源请求标记",http.StatusForbidden);return false}
		if origin:=r.Header.Get("Origin");origin!="" {
			u,err:=url.Parse(origin)
			if err!=nil || !strings.EqualFold(u.Host,r.Host) || (u.Scheme!="http" && u.Scheme!="https") {
				http.Error(w,"非法来源",http.StatusForbidden);return false
			}
		}
		return true
	}
	decode:=func(w http.ResponseWriter,r *http.Request,v any,max int64) bool {
		if !strings.HasPrefix(r.Header.Get("Content-Type"),"application/json") {http.Error(w,"需要 JSON",http.StatusUnsupportedMediaType);return false}
		d:=json.NewDecoder(http.MaxBytesReader(w,r.Body,max))
		d.DisallowUnknownFields()
		if d.Decode(v)!=nil {http.Error(w,"参数格式错误",http.StatusBadRequest);return false}
		var extra any
		if !errors.Is(d.Decode(&extra),io.EOF) {http.Error(w,"仅允许一个对象",http.StatusBadRequest);return false}
		return true
	}
	signed:=func(w http.ResponseWriter,r *http.Request,max int64)(string,string,[]byte,bool){
		body,err:=io.ReadAll(http.MaxBytesReader(w,r.Body,max))
		if err!=nil {http.Error(w,"请求过大",http.StatusRequestEntityTooLarge);return "","",nil,false}
		id,pin,ok:=guard.verify(w,r,body)
		return id,pin,body,ok
	}
	checkPin:=func(w http.ResponseWriter,r *http.Request,id,pin string)(storage.ReverseAgent,bool){
		a,err:=db.ReverseByAgent(r.Context(),id)
		if err!=nil || a.SPKI!=pin {http.Error(w,"Agent 未登记或身份不符",http.StatusForbidden);return storage.ReverseAgent{},false}
		return a,true
	}
	mux.HandleFunc("GET /api/v1/reverse/discovery",func(w http.ResponseWriter,r *http.Request){
		if !admin(w,r,false){return}
		items,err:=db.ListReverseCandidates(r.Context())
		if err!=nil {http.Error(w,"读取发现记录失败",503);return}
		until,err:=db.ReverseDiscoveryUntil(r.Context())
		if err!=nil {http.Error(w,"发现开关不可用",503);return}
		write(w,200,map[string]any{"enabled":until>time.Now().Unix(),"until":until,"agents":items})
	})
	mux.HandleFunc("PUT /api/v1/reverse/discovery",func(w http.ResponseWriter,r *http.Request){
		if !admin(w,r,true){return}
		if len(os.Getenv("EASYNETTOPO_REVERSE_BOOTSTRAP_TOKEN"))<32 {http.Error(w,"先配置反向接入密钥",503);return}
		var req struct{ Enabled bool `json:"enabled"` }
		if !decode(w,r,&req,1024){return}
		until,err:=db.SetReverseDiscovery(r.Context(),req.Enabled)
		if err!=nil {http.Error(w,"更新发现开关失败",503);return}
		write(w,200,map[string]any{"enabled":req.Enabled,"until":until})
	})
	mux.HandleFunc("POST /api/v1/reverse/discovery/{agent_id}/{action}",func(w http.ResponseWriter,r *http.Request){
		if !admin(w,r,true){return}
		id:=r.PathValue("agent_id")
		switch r.PathValue("action"){
		case "accept":
			host,err:=db.AcceptReverse(r.Context(),id)
			if err!=nil {http.Error(w,"主机已被接受或身份冲突",409);return}
			write(w,201,host)
		case "reject":
			if err:=db.RejectReverse(r.Context(),id);err!=nil {http.Error(w,"拒绝失败",503);return}
			write(w,200,map[string]any{"accepted":false})
		default:http.NotFound(w,r)
		}
	})
	mux.HandleFunc("DELETE /api/v1/hosts/{host_id}/temporary",func(w http.ResponseWriter,r *http.Request){
		if !admin(w,r,true){return}
		if err:=db.RevokeReverse(r.Context(),r.PathValue("host_id"));err!=nil {http.Error(w,"临时主机不存在",404);return}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /api/v1/hosts/{host_id}/preinstall",func(w http.ResponseWriter,r *http.Request){
		if !admin(w,r,true){return}
		if catalogSvc==nil {http.Error(w,"应用市场不可用",503);return}
		var req struct{
			SourceID string `json:"source_id"`
			AppID string `json:"app_id"`
			Revision string `json:"revision"`
			Config map[string]string `json:"config"`
		}
		if !decode(w,r,&req,256<<10){return}
		if req.AppID=="easynettopo" {http.Error(w,"不允许通过预安装部署控制端自身",403);return}
		hosts,err:=hostSvc.List(r.Context())
		if err!=nil {http.Error(w,"主机列表不可用",503);return}
		var target *storage.Host
		for i:=range hosts{if hosts[i].ID==r.PathValue("host_id"){target=&hosts[i];break}}
		if target==nil || target.HostType!="temporary" {http.Error(w,"只允许有效的临时主机预安装",409);return}
		pkg,err:=catalogSvc.BuildCandidateBundle(r.Context(),req.SourceID,req.AppID)
		if err!=nil || pkg.Revision!=req.Revision {http.Error(w,"应用清单或固定版本不可用",422);return}
		fields:=make(map[string]map[string]any,len(pkg.Config))
		for _,field:=range pkg.Config{key,_:=field["key"].(string);fields[key]=field}
		for key:=range req.Config{
			field,exists:=fields[key]
			if !exists || !catalogConfigFieldInputAllowed(key,field,fields,req.Config) {
				http.Error(w,"存在不允许的配置参数",422);return
			}
		}
		if catalogRequiredConfigFieldMissing(fields,req.Config)!="" {http.Error(w,"缺少必填安装参数",422);return}
		taskID,err:=db.AddReverseInstall(r.Context(),target.ID,req.SourceID,req.AppID,req.Revision,pkg.SHA256,req.Config,pkg.ZIP)
		if err!=nil {http.Error(w,"创建待领取安装任务失败："+err.Error(),409);return}
		write(w,202,map[string]any{"task_id":taskID,"state":"queued","accepted":true})
	})
	// 所有外网 Agent API 只接受服务端签发的固定 URL，不允许向任意地址转发数据。
	mux.HandleFunc("POST /api/v1/reverse/report",func(w http.ResponseWriter,r *http.Request){
		id,pin,raw,ok:=signed(w,r,4096)
		if !ok{return}
		var req struct{
			Hostname string `json:"hostname"`
			Platform string `json:"platform"`
			Arch string `json:"arch"`
			Version string `json:"version"`
		}
		if json.Unmarshal(raw,&req)!=nil {http.Error(w,"报到数据格式错误",400);return}
		a,err:=db.ReverseReport(r.Context(),storage.ReverseAgent{AgentID:id,SPKI:pin,Hostname:req.Hostname,Platform:req.Platform,Arch:req.Arch,Version:req.Version})
		if errors.Is(err,storage.ErrReverseDisabled) {write(w,403,map[string]any{"code":"discovery_closed"});return}
		if errors.Is(err,storage.ErrReverseInactive) {write(w,410,map[string]any{"code":"registration_expired"});return}
		if err!=nil {http.Error(w,"报到未接受",409);return}
		write(w,200,map[string]any{"status":a.Status,"host_id":a.HostID,"expires_at":a.ExpiresAt,"poll_seconds":storage.ReversePollSeconds})
	})
	mux.HandleFunc("POST /api/v1/reverse/claim",func(w http.ResponseWriter,r *http.Request){
		id,pin,_,ok:=signed(w,r,1024)
		if !ok{return}
		if _,ok=checkPin(w,r,id,pin);!ok{return}
		task,err:=db.ClaimReverse(r.Context(),id)
		if errors.Is(err,storage.ErrReverseInactive){write(w,410,map[string]string{"code":"registration_expired"});return}
		if err!=nil {http.Error(w,"领取任务失败",503);return}
		if err:=db.TouchReverse(r.Context(),id);err!=nil {http.Error(w,"报到状态更新失败",503);return}
		if task==nil {w.WriteHeader(http.StatusNoContent);return}
		write(w,200,task)
	})
	mux.HandleFunc("POST /api/v1/reverse/tasks/{task_id}/ack",func(w http.ResponseWriter,r *http.Request){
		id,pin,_,ok:=signed(w,r,1024)
		if !ok{return}
		if _,ok=checkPin(w,r,id,pin);!ok{return}
		if err:=db.ReverseAck(r.Context(),id,r.PathValue("task_id"));err!=nil {http.Error(w,"任务不允许确认",409);return}
		write(w,200,map[string]bool{"accepted":true})
	})
	mux.HandleFunc("POST /api/v1/reverse/tasks/{task_id}/result",func(w http.ResponseWriter,r *http.Request){
		id,pin,raw,ok:=signed(w,r,4096)
		if !ok{return}
		if _,ok=checkPin(w,r,id,pin);!ok{return}
		var req struct{ State string `json:"state"`; Message string `json:"message"` }
		if json.Unmarshal(raw,&req)!=nil {http.Error(w,"结果格式错误",400);return}
		if err:=db.ReverseComplete(r.Context(),id,r.PathValue("task_id"),req.State,req.Message);err!=nil {http.Error(w,"任务状态不允许变更",409);return}
		write(w,200,map[string]bool{"accepted":true})
	})
	mux.HandleFunc("GET /api/v1/reverse/tasks/{task_id}/bundle",func(w http.ResponseWriter,r *http.Request){
		id,pin,_,ok:=signed(w,r,0)
		if !ok{return}
		if _,ok=checkPin(w,r,id,pin);!ok{return}
		hash,blob,err:=db.ReverseBundle(r.Context(),id,r.PathValue("task_id"))
		if err!=nil {http.Error(w,"应用包不可用",404);return}
		w.Header().Set("Content-Type","application/octet-stream")
		w.Header().Set("X-Bundle-SHA256",hash)
		w.Header().Set("Cache-Control","no-store")
		w.Header().Set("Content-Length",strconv.Itoa(len(blob)))
		_,_=io.Copy(w,bytes.NewReader(blob))
	})
}

