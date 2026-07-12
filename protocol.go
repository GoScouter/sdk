package sdk

// ProtocolVersion is the version of the host<->module wire protocol implemented
// by this SDK. A module binary reports it in its [Descriptor] so the host can
// refuse binaries built against an incompatible protocol.
const ProtocolVersion = 1

// Subcommands understood by a module binary. The host invokes the binary with
// one of these as its first argument.
const (
	// cmdDescribe makes the binary print its Descriptor as JSON and exit.
	cmdDescribe = "describe"
	// cmdScout makes the binary run its module against -target and print the
	// rendered result.
	cmdScout = "scout"
)

// Descriptor is the metadata a module binary reports in response to the
// "describe" command. It is the JSON contract a module binary writes and [Open]
// reads.
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
