// Package sdk defines the contract for building GoScouter modules.
//
// A module is a self-contained scouting capability: given a target, it gathers
// information and returns a renderable result. Developers implement [Module]
// and the GoScouter host application is responsible for discovering, loading,
// and running it.
//
// A module is distributed as a standalone executable. The host discovers,
// downloads, and caches that binary, then loads it with [Open], which returns a
// [Binary] that satisfies [Module] by invoking the executable as a subprocess.
//
// A module binary must implement a small command-line protocol:
//
//	<binary> describe
//	    Write a JSON [Descriptor] to stdout and exit 0.
//
//	<binary> scout -target <target> [module flags...]
//	    Run against target, write the rendered result to stdout, and exit 0.
//	    Any operator-supplied module flags are forwarded verbatim after
//	    -target; the binary parses its own flags. On failure, write a message
//	    to stderr and exit with a non-zero status.
//
// Any executable that honors this protocol is a valid module, regardless of the
// language it is written in. The [Module] and [Result] interfaces describe the
// same contract in Go terms, and are the types the host works with once a binary
// is loaded.
//
// Downloading, caching, verifying, and registering modules is the host's
// responsibility; the SDK owns the interfaces and the wire protocol they travel
// over.
package sdk

// Module is a scouting capability that GoScouter can run against a target.
//
// Implementations must be safe to call concurrently: the host may invoke Scout
// on the same Module value from multiple goroutines.
type Module interface {
	// Name is the unique identifier the module is invoked by. It must be
	// stable, lower-case, and contain no spaces.
	Name() string

	// Description is a one-line human-readable summary shown in help output.
	Description() string

	// Version is the module's own version, independent of the SDK version.
	// Semantic versioning (e.g. "1.2.0") is recommended.
	Version() string

	// Scout gathers information about target and returns a renderable result.
	// target is the raw string supplied by the operator (typically a URL).
	// args carries any module-specific flags the operator passed after the
	// module name (e.g. []string{"--https"}); modules that take no options
	// ignore it.
	Scout(target string, args []string) (Result, error)
}

// Result is the outcome of a [Module.Scout] call. It knows how to present
// itself as terminal-ready text.
type Result interface {
	// Render returns the result formatted for display in the GoScouter
	// terminal. Use "\r\n" line endings, as the host runs in raw mode.
	Render() string
}
