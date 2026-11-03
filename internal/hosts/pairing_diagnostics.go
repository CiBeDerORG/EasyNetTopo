package hosts

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
)

// PairingStageError 标明配对失败发生的阶段；Cause 仅用于进程内诊断，不能输出到浏览器或日志。
type PairingStageError struct {
	Stage string
	Cause error
}

func (e *PairingStageError) Error() string { return fmt.Sprintf("主机配对阶段 %s 失败", e.Stage) }
func (e *PairingStageError) Unwrap() error { return e.Cause }

// PairingHTTPError 保存 Agent 返回的有限错误码，不携带响应正文和任何密钥。
type PairingHTTPError struct {
	Status int
	Code   string
}

func (e *PairingHTTPError) Error() string {
	return fmt.Sprintf("Agent 配对接口 HTTP %d (%s)", e.Status, e.Code)
}

// PairingDiagnostic 映射为有限、可供 UI 显示的诊断消息；不包含 IP、密钥或远端正文。
type PairingDiagnostic struct {
	Status       int    `json:"-"`
	Code         string `json:"code"`
	Stage        string `json:"stage"`
	Message      string `json:"message"`
	AgentStatus  int    `json:"-"`
}

// DiagnosePairing 只公开可信阶段和协议错误码，避免泄露配对码/Token/内部数据库错误。
func DiagnosePairing(err error) PairingDiagnostic {
	diagnostic := PairingDiagnostic{
		Status: 502, Code: "pairing_failed", Stage: "unknown",
		Message: "配对失败，请检查 Agent 连接与主机服务日志",
	}
	var step *PairingStageError
	if errors.As(err, &step) {
		diagnostic.Stage = step.Stage
		switch step.Stage {
		case "identity":
			diagnostic.Code = "identity_failed"
			diagnostic.Message = "无法核对 Agent 身份；请检查主机地址、HTTP/HTTPS、UUID、公钥指纹及控制端到 Agent 的网络"
		case "exchange":
			diagnostic.Code = "pairing_exchange_failed"
			diagnostic.Message = "Agent 配对接口调用失败；请检查 Agent 服务、网络和配对码有效期"
		case "response":
			diagnostic.Code = "pairing_response_invalid"
			diagnostic.Message = "Agent 配对响应校验失败；请重新生成配对码并检查 Agent 版本"
		case "save":
			diagnostic.Code = "credential_save_failed"
			diagnostic.Status = 503
			diagnostic.Message = "Agent 已接受配对，但 EasyNetTopo 保存凭据失败；请检查控制端数据目录，并重新生成配对码"
		case "readback":
			diagnostic.Code = "credential_readback_failed"
			diagnostic.Status = 503
			diagnostic.Message = "Agent 已接受配对，凭据可能已保存，但主机信息读取失败；请先刷新主机列表"
		}
	}
	var remote *PairingHTTPError
	if errors.As(err, &remote) {
		diagnostic.AgentStatus = remote.Status
		diagnostic.Stage = "exchange"
		switch remote.Code {
		case "invalid_pairing_code":
			diagnostic.Status = 422
			diagnostic.Code = remote.Code
			diagnostic.Message = "Agent 拒绝了配对码；请重新核对或在 Agent 主机生成新配对码"
		case "pairing_window_closed":
			diagnostic.Status = 409
			diagnostic.Code = remote.Code
			diagnostic.Message = "配对码不存在、已使用或已过期；请在 Agent 主机生成新配对码"
		case "pairing_attempts_exhausted":
			diagnostic.Status = 429
			diagnostic.Code = remote.Code
			diagnostic.Message = "Agent 配对失败次数已达上限；请在 Agent 主机生成新配对码"
		case "pairing_unavailable":
			diagnostic.Status = 503
			diagnostic.Code = remote.Code
			diagnostic.Message = "Agent 配对存储不可用；请检查 Agent 的 systemd 日志"
		case "invalid_identity":
			diagnostic.Status = 422
			diagnostic.Code = remote.Code
			diagnostic.Message = "Agent 拒绝控制端或主机标识；请检查两端协议版本"
		default:
			diagnostic.Code = "agent_http_error"
			diagnostic.Message = "Agent 配对接口返回异常状态；请检查 Agent 服务日志"
		}
	}
	var urlError *url.Error
	var netError net.Error
	if errors.As(err, &urlError) || errors.As(err, &netError) || errors.Is(err, context.DeadlineExceeded) {
		if diagnostic.Code == "identity_failed" || diagnostic.Code == "pairing_exchange_failed" {
			diagnostic.Code = "agent_unreachable"
			diagnostic.Message = "无法连接 Agent；请确认 EasyNetTopo 所在服务器能访问 Agent 的 IP、端口及 HTTP/HTTPS 协议"
			diagnostic.Status = 502
		}
	}
	return diagnostic
}
