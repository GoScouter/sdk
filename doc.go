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
// A Scout reports failure by returning an error, which reaches the host as an
// ordinary error carrying the module's own message:
//
//	func (whois) Scout(target string, args []string) (json.RawMessage, error) {
//		res, err := lookup(target)
//		if err != nil {
//			return nil, fmt.Errorf("whois %s: %w", target, err)
//		}
//		...
//	}
//
// Only that one call fails; a Scout that panics is contained the same way.
// Either way the module stays up and keeps serving, which is why a Scout must
// never end the process. a log.Fatal on a bad target takes every other call
// in flight down with it, and every call after it. Once the module does exit or
// is closed, calls in flight and calls made afterwards return [ErrClosed].
package sdk
