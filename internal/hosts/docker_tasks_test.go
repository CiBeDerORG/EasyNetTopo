package hosts

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDockerResourceAndPersistentTaskClient(t *testing.T) {
	const controllerID = "22222222-2222-4222-8222-222222222222"
	const taskID = "11111111-1111-4111-8111-111111111111"
	const pin = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	const token = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/docker/resources/compose:test/logs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-EHA-Agent-ID", mockAgentID)
		if r.Method != http.MethodGet || r.URL.Query().Get("tail") != "20" ||
			r.Header.Get("Authorization") != "Bearer "+token ||
			r.Header.Get("X-EHA-Controller-ID") != controllerID {
			http.Error(w, "invalid request", http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"agent_id": mockAgentID, "resource_id": "compose:test", "text": "sample log",
		})
	})
	mux.HandleFunc("/v1/tasks", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-EHA-Agent-ID", mockAgentID)
		if r.Method != http.MethodPost || r.Header.Get("Authorization") != "Bearer "+token ||
			r.Header.Get("X-EHA-Controller-ID") != controllerID {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var task DockerTaskRequest
		if err := json.NewDecoder(r.Body).Decode(&task); err != nil || task.TaskID != taskID ||
			task.Kind != "docker_resource" || task.Action != "stop" || task.ResourceID != "compose:test" {
			http.Error(w, "invalid task", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"agent_id": mockAgentID, "task_id": taskID, "resource_id": task.ResourceID, "status": "accepted",
		})
	})
	mux.HandleFunc("/v1/tasks/"+taskID, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-EHA-Agent-ID", mockAgentID)
		if r.Method != http.MethodGet || r.Header.Get("Authorization") != "Bearer "+token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"agent_id": mockAgentID, "task_id": taskID, "status": "succeeded",
		})
	})
	mux.HandleFunc("/v1/tasks/"+taskID+"/cancel", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-EHA-Agent-ID", mockAgentID)
		if r.Method != http.MethodPost || r.Header.Get("Authorization") != "Bearer "+token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"agent_id": mockAgentID, "task_id": taskID, "status": "cancelled",
		})
	})

	server := httptest.NewServer(mux)
	defer server.Close()
	client, err := NewClient(server.URL, mockAgentID, pin)
	if err != nil {
		t.Fatal(err)
	}
	logs, err := client.DockerResource(context.Background(), controllerID, token, "compose:test", "logs", 20)
	if err != nil || !strings.Contains(string(logs), "sample log") {
		t.Fatalf("Docker logs proxy failed: %s %v", logs, err)
	}
	task := DockerTaskRequest{
		TaskID: taskID, Kind: "docker_resource", Action: "stop", ResourceID: "compose:test",
	}
	created, err := client.StartDockerTask(context.Background(), controllerID, token, task)
	if err != nil || !strings.Contains(string(created), "accepted") {
		t.Fatalf("Docker task create failed: %s %v", created, err)
	}
	status, err := client.GetDockerTask(context.Background(), controllerID, token, taskID)
	if err != nil || !strings.Contains(string(status), "succeeded") {
		t.Fatalf("Docker task query failed: %s %v", status, err)
	}
	cancelled, err := client.CancelDockerTask(context.Background(), controllerID, token, taskID)
	if err != nil || !strings.Contains(string(cancelled), "cancelled") {
		t.Fatalf("Docker task cancel failed: %s %v", cancelled, err)
	}
	if _, err := client.StartDockerTask(context.Background(), controllerID, token,
		DockerTaskRequest{TaskID: taskID, Kind: "docker_resource", Action: "delete", ResourceID: "compose:test"}); err == nil {
		t.Fatal("unsafe Docker delete action was accepted")
	}
	if _, err := client.DockerResource(context.Background(), controllerID, token, "compose:test/escape", "stats", 100); err == nil {
		t.Fatal("path traversal resource ID was accepted")
	}
}
