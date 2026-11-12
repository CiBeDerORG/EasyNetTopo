package storage

import (
    "context"
    "crypto/sha256"
    "encoding/hex"
    "errors"
    "path/filepath"
    "testing"
    "time"
)

func newReverseTestStore(t *testing.T) (*Store, context.Context) {
    t.Helper()
    ctx := context.Background()
    db, err := Open(filepath.Join(t.TempDir(), "app", "state.db"))
    if err != nil { t.Fatal(err) }
    t.Cleanup(func(){ _ = db.Close() })
    if _, err = NewHostStore(ctx, db); err != nil { t.Fatal(err) }
    if _, err = NewTaskStore(ctx, db); err != nil { t.Fatal(err) }
    if err = db.InitReverse(ctx); err != nil { t.Fatal(err) }
    return db, ctx
}

func reverseTestAgent() ReverseAgent {
    return ReverseAgent{
        AgentID: "53ccfc20-e965-4afc-9927-c2af86138355",
        SPKI: "e4b574591d364b8a5a5adfa91d7c419d404588ae83071ac553df9912b3b26bee",
        Hostname: "private-test", Platform: "linux", Arch: "amd64", Version: "test",
    }
}

func TestReverseTemporaryTTLNeverRenews(t *testing.T) {
    db, ctx := newReverseTestStore(t)
    agent := reverseTestAgent()
    if _, err := db.ReverseReport(ctx, agent); !errors.Is(err, ErrReverseDisabled) {
        t.Fatalf("发现未开启也允许登记: %v", err)
    }
    if _, err := db.SetReverseDiscovery(ctx, true); err != nil { t.Fatal(err) }
    if _, err := db.ReverseReport(ctx, agent); err != nil { t.Fatal(err) }
    host, err := db.AcceptReverse(ctx, agent.AgentID)
    if err != nil { t.Fatal(err) }
    if host.ConnectionMode != "reverse" || host.HostType != "temporary" { t.Fatalf("主机类型异常: %+v", host) }
    previous, err := db.ReverseByAgent(ctx, agent.AgentID)
    if err != nil { t.Fatal(err) }
    for i := 0; i < 4; i++ {
        if _, err = db.ReverseReport(ctx, agent); err != nil { t.Fatal(err) }
        if err = db.TouchReverse(ctx, agent.AgentID); err != nil { t.Fatal(err) }
    }
    after, err := db.ReverseByAgent(ctx, agent.AgentID)
    if err != nil { t.Fatal(err) }
    if previous.AcceptedAt != after.AcceptedAt || previous.ExpiresAt != after.ExpiresAt {
        t.Fatalf("轮询或报到延长有效期: before=%+v after=%+v", previous, after)
    }
    if after.ExpiresAt - after.AcceptedAt != int64(ReverseTTL/time.Second) {
        t.Fatalf("临时主机不是固定 30 分钟: %+v", after)
    }
    _, err = db.db.ExecContext(ctx,"UPDATE reverse_agents SET expires_at=? WHERE agent_id=?",time.Now().Add(-time.Second).Unix(),agent.AgentID)
    if err != nil { t.Fatal(err) }
    if err = db.ExpireReverse(ctx); err != nil { t.Fatal(err) }
    hs,err := NewHostStore(ctx,db)
    if err != nil { t.Fatal(err) }
    hosts,err := hs.List(ctx)
    if err != nil { t.Fatal(err) }
    if len(hosts) != 0 { t.Fatalf("过期临时主机未从主机列表清理: %+v",hosts) }
    if _,err = db.ReverseReport(ctx,agent); !errors.Is(err,ErrReverseInactive){
        t.Fatalf("过期 Agent 可以重新报到续期: %v",err)
    }
}

func TestReverseJobClaimACKCompleteIdempotent(t *testing.T) {
    db,ctx:=newReverseTestStore(t)
    agent:=reverseTestAgent()
    if _,err:=db.SetReverseDiscovery(ctx,true);err!=nil{t.Fatal(err)}
    if _,err:=db.ReverseReport(ctx,agent);err!=nil{t.Fatal(err)}
    h,err:=db.AcceptReverse(ctx,agent.AgentID)
    if err!=nil{t.Fatal(err)}
    bundle:=[]byte("unit-test-bundle")
    sum:=sha256.Sum256(bundle)
    digest:=hex.EncodeToString(sum[:])
    id,err:=db.AddReverseInstall(ctx,h.ID,"323e4cba-cfc8-4989-9fd2-b6a0bd33b6ed","easytier","deadbeef",digest,
       map[string]string{"NETWORK_SECRET":"do-not-log"},bundle)
    if err!=nil{t.Fatal(err)}
    first,err:=db.ClaimReverse(ctx,agent.AgentID)
    if err!=nil || first==nil || first.TaskID!=id || first.Config["NETWORK_SECRET"]!="do-not-log" {
       t.Fatalf("首次领取内容错误: %+v error=%v",first,err)
    }
    again,err:=db.ClaimReverse(ctx,agent.AgentID)
    if err!=nil || again==nil || again.TaskID!=first.TaskID {
       t.Fatalf("相同任务必须返回同一 ID: %+v err=%v",again,err)
    }
    if err:=db.ReverseAck(ctx,agent.AgentID,id);err!=nil{t.Fatal(err)}
    if err:=db.ReverseAck(ctx,agent.AgentID,id);err!=nil{t.Fatalf("重复 ACK 不应失败: %v",err)}
    hash,got,err:=db.ReverseBundle(ctx,agent.AgentID,id)
    if err!=nil || hash!=digest || string(got)!=string(bundle){t.Fatalf("下载包不一致: %v",err)}
    if err:=db.ReverseComplete(ctx,agent.AgentID,id,"succeeded","");err!=nil{t.Fatal(err)}
    if err:=db.ReverseComplete(ctx,agent.AgentID,id,"succeeded","");err!=nil{t.Fatalf("重复回报应幂等: %v",err)}
    nothing,err:=db.ClaimReverse(ctx,agent.AgentID)
    if err!=nil || nothing!=nil{t.Fatalf("已完成任务被重新领取: %+v err=%v",nothing,err)}
}
