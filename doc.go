// Package sdk implements the gs module protocol: how a module binary and the
// host that runs it talk to each other.
//
// A module is a standalone executable that answers questions about a target.
// It implements [Module] and hands itself to [Serve] from main:
//
//	func main() {
//		if err := sdk.Serve(whois{}); err != nil {
//			log.Fatal(err)
//		}
//	}
//
// The host side is [Open], which starts such a binary and speaks to it over
// the subprocess pipes:
//
//	m, err := sdk.Open("./modules/whois")
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer m.Close()
//
//	raw, err := m.Scout(ctx, "example.com", nil)
//
// # The wire
//
// Requests and responses are JSON objects carried on the module's stdin and
// stdout. Every request has an id and the matching response repeats it, so
// both sides may keep several calls in flight: [Binary] multiplexes concurrent
// [Binary.Scout] calls over the one subprocess, and [Serve] handles each
// request in its own goroutine. A module's Scout must therefore be safe for
// concurrent use.
//
// Because the protocol owns stdout, a module must never print to it. Stderr is
// free, and is passed through to the host's own stderr.
//
// # Failure
//
// A Scout that panics is reported to the host as an error on that one call;
// the module itself stays up. Once the module exits or is closed, calls in
// flight and calls made afterwards return [ErrClosed]. Errors raised by the
// module arrive as ordinary errors carrying the module's own message.
package sdk
