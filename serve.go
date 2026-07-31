package sdk

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
)

// Serve runs m as a module binary, reading requests from stdin and writing
// responses to stdout until stdin is closed. It is the entry point a module's
// main should end with:
//
//	func main() {
//		if err := sdk.Serve(myModule{}); err != nil {
//			log.Fatal(err)
//		}
//	}
//
// Each request is handled in its own goroutine, so a slow Scout does not stall
// the ones behind it. m.Scout must therefore be safe for concurrent use.
//
// Anything the module writes to stdout itself would corrupt the protocol; write
// diagnostics to stderr instead.
func Serve(m Module) error {
	return serve(m, os.Stdin, os.Stdout)
}

func serve(m Module, r io.Reader, w io.Writer) error {
	dec := json.NewDecoder(r)
	enc := json.NewEncoder(w)

	var (
		wmu      sync.Mutex // one writer at a time on stdout
		inFlight sync.WaitGroup
	)

	for {
		var req pipeRequest
		if err := dec.Decode(&req); err != nil {
			// Let outstanding work finish and report before we go away.
			inFlight.Wait()
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("sdk: decode request: %w", err)
		}

		inFlight.Add(1)
		go func() {
			defer inFlight.Done()

			res := handle(m, &req)

			wmu.Lock()
			defer wmu.Unlock()
			enc.Encode(res)
		}()
	}
}

func handle(m Module, req *pipeRequest) (res *pipeResponse) {
	defer func() {
		if r := recover(); r != nil {
			res = &pipeResponse{ID: req.ID, Error: fmt.Sprintf("panic: %v", r)}
		}
	}()

	res = &pipeResponse{ID: req.ID}

	switch req.Method {
	case MethodInfo:
		raw, err := json.Marshal(m.Info())
		if err != nil {
			res.Error = fmt.Sprintf("encode info: %v", err)
			return res
		}
		res.Result = raw
		res.View = ""

	case MethodScout:
		raw, err := m.Scout(req.Target, req.Args)
		if err != nil {
			res.Error = errMessage(err)
			return res
		}
		if len(raw) == 0 {
			raw = json.RawMessage("null")
		}
		if !json.Valid(raw) {
			res.Error = "scout returned invalid JSON"
			return res
		}
		res.Result = raw
		res.View = m.Render(raw)
	default:
		res.Error = "unknown method: " + req.Method
	}

	return res
}

func errMessage(err error) string {
	if msg := err.Error(); msg != "" {
		return msg
	}
	return "scout failed"
}
