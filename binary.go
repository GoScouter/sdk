package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
)

// ErrClosed is returned by calls made against a module that has exited or been
// closed.
var ErrClosed = errors.New("sdk: module is not running")

type Binary struct {
	path string

	stdin  io.WriteCloser
	stdout io.ReadCloser

	cmd *exec.Cmd
	mu  *sync.Mutex

	dec *json.Decoder
	enc *json.Encoder

	// wmu serializes writes to stdin so concurrent calls cannot interleave
	// halves of two requests on the pipe. mu guards everything below it.
	wmu sync.Mutex

	nextID  uint64
	pending map[uint64]chan *pipeResponse

	done    chan struct{}
	closed  bool
	exitErr error
}

// Open starts path as a module subprocess and begins reading its responses.
// The returned Binary is safe for concurrent use: Scout may be called from many
// goroutines at once and the calls are multiplexed over the single subprocess.
func Open(path string) (*Binary, error) {
	cmd := exec.Command(path)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("sdk: stdin pipe for %s: %w", path, err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("sdk: stdout pipe for %s: %w", path, err)
	}
	cmd.Stderr = os.Stderr

	b := &Binary{
		path: path,

		stdin:  stdin,
		stdout: stdout,

		cmd: cmd,
		mu:  &sync.Mutex{},

		dec: json.NewDecoder(stdout),
		enc: json.NewEncoder(stdin),

		pending: make(map[uint64]chan *pipeResponse),
		done:    make(chan struct{}),
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("sdk: start %s: %w", path, err)
	}
	go b.read()

	return b, nil
}

func (b *Binary) Path() string { return b.path }

func (b *Binary) Info(ctx context.Context) (ModuleInfo, error) {
	raw, err := b.call(ctx, &pipeRequest{Method: methodInfo})
	if err != nil {
		return ModuleInfo{}, err
	}
	var info ModuleInfo
	if err := json.Unmarshal(raw, &info); err != nil {
		return ModuleInfo{}, fmt.Errorf("sdk: decode info from %s: %w", b.path, err)
	}
	return info, nil
}

// Scout runs the module's Scout against target. It blocks until the module
// answers, ctx is cancelled, or the module exits -- but it does not block other
// Scout calls, so callers may fan out freely over one Binary.
func (b *Binary) Scout(ctx context.Context, target string, args []string) (json.RawMessage, error) {
	return b.call(ctx, &pipeRequest{Method: methodScout, Target: target, Args: args})
}

func (b *Binary) Close() error {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		<-b.done
		return nil
	}
	b.closed = true
	b.mu.Unlock()
	b.stdin.Close()
	<-b.done

	if err := b.cmd.Wait(); err != nil {
		return fmt.Errorf("sdk: %s exited: %w", b.path, err)
	}
	return nil
}

func (b *Binary) call(ctx context.Context, req *pipeRequest) (json.RawMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	ch := make(chan *pipeResponse, 1)

	b.mu.Lock()
	if b.closed || b.exitErr != nil {
		err := b.exitErr
		b.mu.Unlock()
		if err == nil {
			err = ErrClosed
		}
		return nil, err
	}
	b.nextID++
	req.ID = b.nextID
	b.pending[req.ID] = ch
	b.mu.Unlock()

	b.wmu.Lock()
	err := b.enc.Encode(req)
	b.wmu.Unlock()
	if err != nil {
		b.forget(req.ID)
		return nil, fmt.Errorf("sdk: send %s to %s: %w", req.Method, b.path, err)
	}

	select {
	case res, ok := <-ch:
		if !ok {
			return nil, b.failure()
		}
		if res.Error != "" {
			return nil, fmt.Errorf("sdk: %s %s: %s", b.path, req.Method, res.Error)
		}
		return res.Result, nil
	case <-ctx.Done():
		b.forget(req.ID)
		return nil, ctx.Err()
	}
}

func (b *Binary) read() {
	defer close(b.done)

	for {
		var res pipeResponse
		if err := b.dec.Decode(&res); err != nil {
			b.fail(err)
			return
		}

		b.mu.Lock()
		ch, ok := b.pending[res.ID]
		delete(b.pending, res.ID)
		b.mu.Unlock()

		if ok {
			ch <- &res // buffered, and delivered at most once
		}
	}
}

func (b *Binary) forget(id uint64) {
	b.mu.Lock()
	delete(b.pending, id)
	b.mu.Unlock()
}

func (b *Binary) fail(err error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.exitErr == nil {
		switch {
		case b.closed, errors.Is(err, io.EOF), errors.Is(err, os.ErrClosed):
			b.exitErr = ErrClosed
		default:
			b.exitErr = fmt.Errorf("sdk: read from %s: %w", b.path, err)
		}
	}
	for id, ch := range b.pending {
		delete(b.pending, id)
		close(ch)
	}
}

func (b *Binary) failure() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.exitErr != nil {
		return b.exitErr
	}
	return ErrClosed
}
