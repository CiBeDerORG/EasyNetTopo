package storage

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"math/big"
	"path/filepath"
	"testing"
	"time"
)

func testReverseIdentity(t *testing.T, agentID string) ([]byte, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	cert := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{CommonName: "EasyHostAgent " + agentID},
		NotBefore: now.Add(-time.Minute),
		NotAfter: now.Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, &cert, &cert, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	spki, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(spki)
	return der, hex.EncodeToString(digest[:])
}

func TestReverseDiscoveryAndPreinstallLifecycle(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "reverse-test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	hosts, err := NewHostStore(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	tasks, err := NewTaskStore(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	direct, err := hosts.Create(ctx, "permanent", "http://127.0.0.1:7014")
	if err != nil {
		t.Fatal(err)
	}
	token, state, err := hosts.OpenDiscovery(ctx)
	if err != nil || !state.Enabled || len(token) != 64 {
		t.Fatalf("open discovery state=%+v err=%v", state, err)
	}
	agentID, err := uuid()
	if err != nil {
		t.Fatal(err)
	}
	der, fingerprint := testReverseIdentity(t, agentID)
	agent := DiscoveredAgent{
		AgentID: agentID, Hostname: "internal-node", SPKISHA256: fingerprint,
		Platform: "linux", Arch: "amd64", Version: "test",
	}
	status, err := hosts.RegisterDiscovery(ctx, token, agent, der)
	if err != nil || status != "pending" {
		t.Fatalf("register discovery status=%q err=%v", status, err)
	}
	if err := hosts.CloseDiscovery(ctx); err != nil {
		t.Fatal(err)
	}
	pending, err := hosts.ListDiscoveries(ctx)
	if err != nil || len(pending) != 1 {
		t.Fatalf("pending=%v err=%v", pending, err)
	}
	h, agentToken, err := hosts.AcceptDiscovery(ctx, agentID)
	if err != nil || h.HostType != "temporary" || h.ConnectionMode != "reverse" || len(agentToken) != 64 {
		t.Fatalf("accept host=%+v token-length=%d err=%v", h, len(agentToken), err)
	}
	status, err = hosts.RegisterDiscovery(ctx, token, agent, der)
	if err != nil || status != "accepted" {
		t.Fatalf("accepted rediscovery status=%q err=%v", status, err)
	}
	if _, err := hosts.UpdateAgentPoll(ctx, agentID, 10*time.Second); err != nil {
		t.Fatalf("agent poll: %v", err)
	}
	bundle := []byte("reverse test bundle")
	digest := sha256.Sum256(bundle)
	digestText := hex.EncodeToString(digest[:])
	created, err := tasks.EnqueueReverseInstall(ctx, h.ID, "source", "example", "rev1",
		digestText, map[string]string{"VAR": "value"}, bundle)
	if err != nil || len(created) != 2 {
		t.Fatalf("enqueue=%+v err=%v", created, err)
	}
	pendingHosts, err := tasks.PendingHosts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range pendingHosts {
		if id == h.ID {
			t.Fatal("reverse host incorrectly dispatched through direct scheduler")
		}
	}
	claimed, err := tasks.ClaimReverseTask(ctx, agentID)
	if err != nil || claimed == nil || claimed.Config["VAR"] != "value" {
		t.Fatalf("claimed=%+v err=%v", claimed, err)
	}
	if _, err := tasks.AcknowledgeReverseTask(ctx, agentID, claimed.Task.ID); err != nil {
		t.Fatalf("ack: %v", err)
	}
	if received, hash, err := tasks.ReverseBundle(ctx, agentID, claimed.Task.ID); err != nil ||
		hash != digestText || string(received) != string(bundle) {
		t.Fatalf("bundle hash=%q bytes=%q err=%v", hash, received, err)
	}
	if err := tasks.UpdateReverseTaskState(ctx, agentID, claimed.Task.ID, "succeeded", ""); err != nil {
		t.Fatalf("task success: %v", err)
	}
	if err := hosts.RevokeTemporary(ctx, h.ID); err != nil {
		t.Fatal(err)
	}
	if hosts.TemporaryActive(h) {
		// The object is an old snapshot: retrieve current state.
		current, err := hosts.Get(ctx, h.ID)
		if err != nil || hosts.TemporaryActive(current) {
			t.Fatalf("host should be revoked: %+v err=%v", current, err)
		}
	}
	if _, err := hosts.Get(ctx, direct.ID); err != nil {
		t.Fatalf("direct permanent host must remain unchanged: %v", err)
	}
}
