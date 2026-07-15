package sdk

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
)

// Serve runs m as a module binary, speaking the stdio protocol on this
// process's stdin and stdout. It is the entire body a module author needs in
// main:
//
//	func main() {
//		if err := sdk.Serve(myModule{}); err != nil {
//			log.Fatal(err)
//		}
//	}
//
// Serve reads one JSON request per line from stdin and writes one JSON response
// per line to stdout, dispatching each request in its own goroutine so a single
// process can service many targets concurrently. Because requests run
// concurrently, m must be safe for concurrent use, as [Module] requires. Serve
// returns when stdin reaches EOF, which the host triggers by closing the
// session.
func Serve(m Module) error {
	return serve(m, os.Stdin, os.Stdout)
}

func serve(m Module, in io.Reader, out io.Writer) error {
	var wmu sync.Mutex
	enc := json.NewEncoder(out)
	write := func(resp response) {
		wmu.Lock()
		_ = enc.Encode(&resp)
		wmu.Unlock()
	}

	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, 64*1024), maxLine)

	var wg sync.WaitGroup
	for scanner.Scan() {
		line := append([]byte(nil), scanner.Bytes()...)

		var req request
		if err := json.Unmarshal(line, &req); err != nil {
			continue // ignore a malformed line; the host correlates by ID anyway
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			write(dispatch(m, req))
		}()
	}

	wg.Wait()
	return scanner.Err()
}

// dispatch runs a single request against m and builds its response.
func dispatch(m Module, req request) (resp response) {
	resp.ID = req.ID
	defer func() {
		if r := recover(); r != nil {
			resp = response{ID: req.ID, Error: fmt.Sprintf("module panicked: %v", r)}
		}
	}()

	switch req.Method {
	case methodDescribe:
		resp.Descriptor = &Descriptor{
			Protocol:    ProtocolVersion,
			Name:        m.Name(),
			Description: m.Description(),
			Version:     m.Version(),
		}
	case methodScout:
		res, err := m.Scout(req.Target, req.Args)
		if err != nil {
			resp.Error = err.Error()
			return resp
		}
		resp.Result = res.Render()
	default:
		resp.Error = fmt.Sprintf("sdk: unknown method %q", req.Method)
	}
	return resp
}
