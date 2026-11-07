package hosts

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestPairingDiagnosticsArePreciseAndSafe(t *testing.T) {
	for _, tt := range []struct {
		stage, agentCode, expected string
		status                     int
	}{
		{"exchange", "invalid_pairing_code", "invalid_pairing_code", 422},
		{"exchange", "pairing_window_closed", "pairing_window_closed", 409},
		{"exchange", "pairing_attempts_exhausted", "pairing_attempts_exhausted", 429},
		{"exchange", "pairing_unavailable", "pairing_unavailable", 503},
		{"exchange", "invalid_identity", "invalid_identity", 422},
		{"exchange", "untrusted_secret_value", "agent_http_error", 502},
	} {
		err := &PairingStageError{
			Stage: tt.stage,
			Cause: &PairingHTTPError{Status: 403, Code: tt.agentCode},
		}
		result := DiagnosePairing(err)
		if result.Code != tt.expected || result.Status != tt.status || result.Stage != tt.stage || result.AgentStatus != 403 {
			t.Fatalf("配对错误码映射不正确 %+v", result)
		}
		raw, marshalErr := json.Marshal(result)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		if strings.Contains(string(raw), "untrusted_secret_value") || strings.Contains(string(raw), "AgentStatus") {
			t.Fatal("配对诊断泄露了未受信任的远端内容")
		}
	}
	for _, tt := range []struct {
		stage, want string
	}{
		{"input", "invalid_pairing_input"},
		{"identity", "identity_failed"},
		{"response", "pairing_response_invalid"},
		{"save", "credential_save_failed"},
		{"readback", "credential_readback_failed"},
	} {
		result := DiagnosePairing(&PairingStageError{Stage: tt.stage, Cause: errors.New("private_database_password_value")})
		if result.Code != tt.want || strings.Contains(result.Message, "private_database_password_value") {
			t.Fatalf("配对阶段诊断不正确或泄露隐私：%+v", result)
		}
	}
}
