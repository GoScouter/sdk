package sdk

// ProtocolVersion is the version of the host<->module wire protocol implemented
// by this SDK. A module binary reports it in its [Descriptor] so the host can
// refuse binaries built against an incompatible protocol.
const ProtocolVersion = 3

// Wire method names carried in a [request].
const (
	// methodDescribe asks the module for its [Descriptor].
	methodDescribe = "describe"
	// methodScout asks the module to run against a target.
	methodScout = "scout"
)

// Descriptor is the metadata a module reports in response to a describe request.
// It is the JSON contract a module writes and [Open] reads during the handshake.
type Descriptor struct {
	// Protocol is the wire-protocol version the binary was built against.
	Protocol int `json:"protocol"`
	// Name is the unique identifier the module is invoked by.
	Name string `json:"name"`
	// Description is a one-line human-readable summary.
	Description string `json:"description"`
	// Version is the module's own version.
	Version string `json:"version"`
}

// request is one host->module message. Exactly one is encoded per line on the
// module's stdin. ID correlates the eventual [response]; the host may have many
// requests in flight at once, so IDs let it match replies to callers.
type request struct {
	ID     uint64   `json:"id"`
	Method string   `json:"method"`
	Target string   `json:"target,omitempty"`
	Args   []string `json:"args,omitempty"`
}

// response is one module->host message, encoded one per line on stdout. ID
// echoes the request it answers. Exactly one of Descriptor, Result, or Error is
// meaningful, depending on the request's method and outcome.
type response struct {
	ID         uint64      `json:"id"`
	Descriptor *Descriptor `json:"descriptor,omitempty"`
	Result     string      `json:"result,omitempty"`
	Error      string      `json:"error,omitempty"`
}
