package sdk

import "encoding/json"

const (
	methodInfo  = "info"
	methodScout = "scout"
	methodAsk   = "ask"
)

type askRequest ModuleNamespace

type request struct {
	Type      string          `json:"type"`
	ClientID  string          `json:"client_id"`
	RequestID uint64          `json:"request_id"`
	Data      json.RawMessage `json:"data"`
}

type response struct {
	ClientID  string          `json:"client_id"`
	RequestID uint64          `json:"request_id"`
	Data      json.RawMessage `json:"data"`
	Error     string          `json:"error"`
}

type pipeRequest struct {
	ID     uint64   `json:"id"`
	Method string   `json:"method"`
	Target string   `json:"target,omitempty"`
	Args   []string `json:"args,omitempty"`
}

// A module binary must not write anything else to stdout -- use stderr for
// logging, it is passed through to the host's stderr.
type pipeResponse struct {
	ID     uint64          `json:"id"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}
