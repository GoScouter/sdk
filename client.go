package sdk

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
)

// execCommand is indirected so tests can substitute a fake subprocess.
var execCommand = exec.Command

// maxLine bounds a single JSON message read from a module. Results are
// JSON-escaped onto one line, so this caps the size of one rendered result.
const maxLine = 16 << 20 // 16 MiB

// errClosed is returned by calls made after the session has been shut down.
var errClosed = errors.New("sdk: module session closed")

// Binary is a handle to a running module binary. It implements [Module] by
// speaking the stdio protocol to a single long-lived subprocess: the process is
// spawned once by [Open] and reused for every [Binary.Scout] call until
// [Binary.Close]. This keeps a scan of many targets to one subprocess per
// module instead of one per target.
//
// Downloading, verifying, and caching the binary on disk is the host's
// responsibility; Binary only manages the process and the protocol once the
// executable is present at path. A Binary is safe for concurrent use: Scout may
// be called from many goroutines at once.
type Binary struct {
	path string
	desc Descriptor

	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stderr *lockedBuffer

	mu      sync.Mutex
	enc     *json.Encoder // writes to stdin; guarded by mu
	nextID  uint64
	pending map[uint64]chan response
	err     error // terminal error once the session dies; nil while healthy

	done chan struct{} // closed when the read loop exits
}

// Open spawns the module binary at path and completes a describe handshake. It
// starts the process, reads the module's [Descriptor], and rejects binaries
// built against an incompatible [ProtocolVersion]. The process stays running
// until [Binary.Close]; callers must Close every Binary they Open.
func Open(path string) (*Binary, error) {
	cmd := execCommand(path)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("sdk: stdin pipe for %s: %w", path, err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("sdk: stdout pipe for %s: %w", path, err)
	}
	stderr := &lockedBuffer{}
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("sdk: starting %s: %w", path, err)
	}

	b := &Binary{
		path:    path,
		cmd:     cmd,
		stdin:   stdin,
		stderr:  stderr,
		enc:     json.NewEncoder(stdin),
		pending: make(map[uint64]chan response),
		done:    make(chan struct{}),
	}
	go b.readLoop(stdout)

	resp, err := b.call(request{Method: methodDescribe})
	if err != nil {
		b.Close()
		return nil, fmt.Errorf("sdk: describe %s: %w", path, err)
	}
	if resp.Descriptor == nil {
		b.Close()
		return nil, fmt.Errorf("sdk: %s did not return a descriptor", path)
	}
	d := *resp.Descriptor
	if d.Protocol != ProtocolVersion {
		b.Close()
		return nil, fmt.Errorf("sdk: %s uses protocol %d, host requires %d", path, d.Protocol, ProtocolVersion)
	}
	if d.Name == "" {
		b.Close()
		return nil, fmt.Errorf("sdk: %s reported an empty module name", path)
	}

	b.desc = d
	return b, nil
}

// Path returns the filesystem path of the underlying binary.
func (b *Binary) Path() string { return b.path }

func (b *Binary) Name() string        { return b.desc.Name }
func (b *Binary) Description() string { return b.desc.Description }
func (b *Binary) Version() string     { return b.desc.Version }

// Scout asks the module to run against target and returns its rendered output.
// Module-specific args are forwarded verbatim. Scout is safe to call
// concurrently from multiple goroutines on the same Binary.
func (b *Binary) Scout(target string, args []string) (Result, error) {
	resp, err := b.call(request{Method: methodScout, Target: target, Args: args})
	if err != nil {
		return nil, fmt.Errorf("sdk: scout %q via %s: %w", target, b.desc.Name, err)
	}
	return rawResult(resp.Result), nil
}

// Close shuts the session down: it signals the module to exit by closing its
// stdin, waits for the read loop to drain, and reaps the process. It is safe to
// call more than once.
func (b *Binary) Close() error {
	b.mu.Lock()
	if b.err == nil {
		b.err = errClosed
	}
	b.mu.Unlock()

	_ = b.stdin.Close() // EOF tells the module to stop reading requests
	<-b.done            // wait for readLoop to finish reading stdout
	return b.cmd.Wait()
}

// call sends one request and blocks until its response arrives or the session
// dies. Writes are serialized under mu, which also makes ID assignment atomic
// and guarantees a caller is registered in pending before its request is sent.
func (b *Binary) call(req request) (response, error) {
	ch := make(chan response, 1)

	b.mu.Lock()
	if b.err != nil {
		b.mu.Unlock()
		return response{}, b.err
	}
	b.nextID++
	req.ID = b.nextID
	b.pending[req.ID] = ch
	if err := b.enc.Encode(&req); err != nil {
		delete(b.pending, req.ID)
		b.mu.Unlock()
		return response{}, err
	}
	b.mu.Unlock()

	resp := <-ch
	if resp.Error != "" {
		return resp, errors.New(resp.Error)
	}
	return resp, nil
}

// readLoop decodes responses from the module's stdout and routes each to the
// waiting caller by ID. When the stream ends (the process exited or crashed) it
// fails every outstanding and future call.
func (b *Binary) readLoop(stdout io.Reader) {
	defer close(b.done)

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), maxLine)
	for scanner.Scan() {
		var resp response
		if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
			continue // ignore a malformed line rather than tear down the session
		}

		b.mu.Lock()
		ch, ok := b.pending[resp.ID]
		if ok {
			delete(b.pending, resp.ID)
		}
		b.mu.Unlock()

		if ok {
			ch <- resp
		}
	}

	b.fail(b.exitError(scanner.Err()))
}

// fail records a terminal error and unblocks every pending caller with it. Once
// failed, the session refuses new calls in call().
func (b *Binary) fail(err error) {
	b.mu.Lock()
	if b.err == nil {
		b.err = err
	}
	pending := b.pending
	b.pending = make(map[uint64]chan response)
	b.mu.Unlock()

	for _, ch := range pending {
		ch <- response{Error: err.Error()}
	}
}

// exitError builds the error reported to callers when the stream ends. A closed
// session is expected (errClosed); otherwise the module died unexpectedly and
// its stderr, if any, is the most useful detail.
func (b *Binary) exitError(readErr error) error {
	b.mu.Lock()
	closed := b.err == errClosed
	b.mu.Unlock()
	if closed {
		return errClosed
	}

	msg := strings.TrimSpace(b.stderr.String())
	switch {
	case msg != "":
		return fmt.Errorf("sdk: module %s exited: %s", b.path, msg)
	case readErr != nil:
		return fmt.Errorf("sdk: module %s stream error: %w", b.path, readErr)
	default:
		return fmt.Errorf("sdk: module %s exited", b.path)
	}
}

// rawResult is a Result whose rendering is the text produced by a module.
type rawResult string

func (r rawResult) Render() string { return string(r) }

// lockedBuffer is a bytes.Buffer safe for the concurrent writes exec performs
// while draining stderr alongside our reads for diagnostics.
type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}
