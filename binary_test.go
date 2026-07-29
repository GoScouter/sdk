package sdk_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/GoScouter/sdk"
)

const testModuleSrc = `package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"sdk"
)

type mod struct{}

func (mod) Info() sdk.ModuleInfo {
	return sdk.ModuleInfo{Name: "test", Author: "me", Description: "test module"}
}

func (mod) Scout(target string, args []string) json.RawMessage {
	if target == "boom" {
		panic("scout exploded")
	}
	time.Sleep(200 * time.Millisecond)
	return json.RawMessage(fmt.Sprintf("{%q:%q,%q:%d}", "target", target, "args", len(args)))
}

func (mod) Render(raw json.RawMessage) string {
    return string(raw)
}

func main() {
	if err := sdk.Serve(mod{}); err != nil {
		log.Fatal(err)
	}
}
`

func sdkGoVersion(t *testing.T, sdkDir string) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(sdkDir, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), "go "); ok {
			return strings.TrimSpace(v)
		}
	}
	t.Fatalf("no go directive in %s/go.mod", sdkDir)
	return ""
}

func buildModule(t *testing.T) string {
	t.Helper()

	sdkDir, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("main.go", testModuleSrc)
	write("go.mod", fmt.Sprintf("module testmod\n\ngo %s\n\nrequire sdk v0.0.0\n\nreplace sdk => %s\n", sdkGoVersion(t, sdkDir), sdkDir))

	bin := filepath.Join(dir, "testmod")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("building test module: %v\n%s", err, out)
	}
	return bin
}

func open(t *testing.T) *sdk.Binary {
	t.Helper()
	b, err := sdk.Open(buildModule(t))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { b.Close() })
	return b
}

func TestInfo(t *testing.T) {
	b := open(t)

	info, err := b.Info(context.Background())
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if info.Name != "test" || info.Author != "me" {
		t.Fatalf("unexpected info: %+v", info)
	}
}

func TestScoutConcurrent(t *testing.T) {
	b := open(t)

	const n = 20
	var wg sync.WaitGroup
	errs := make(chan error, n)

	start := time.Now()
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()

			target := fmt.Sprintf("host-%d", i)
			raw, err := b.Scout(context.Background(), target, []string{"a", "b"})
			if err != nil {
				errs <- fmt.Errorf("scout %s: %w", target, err)
				return
			}

			var got struct {
				Target string `json:"target"`
				Args   int    `json:"args"`
			}
			if err := json.Unmarshal(raw, &got); err != nil {
				errs <- fmt.Errorf("scout %s: bad payload %s: %w", target, raw, err)
				return
			}
			// A response routed to the wrong caller shows up right here.
			if got.Target != target || got.Args != 2 {
				errs <- fmt.Errorf("scout %s: got %+v", target, got)
			}
		}()
	}
	wg.Wait()
	elapsed := time.Since(start)

	close(errs)
	for err := range errs {
		t.Error(err)
	}

	if elapsed > 2*time.Second {
		t.Errorf("calls did not overlap: %d scouts took %v", n, elapsed)
	}
}

func TestScoutPanicIsolated(t *testing.T) {
	b := open(t)

	if _, err := b.Scout(context.Background(), "boom", nil); err == nil {
		t.Fatal("expected an error from a panicking Scout")
	}

	// The module must still be serving after one call blew up.
	if _, err := b.Scout(context.Background(), "fine", nil); err != nil {
		t.Fatalf("module did not survive a panicking Scout: %v", err)
	}
}

func TestScoutContextCancel(t *testing.T) {
	b := open(t)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	if _, err := b.Scout(ctx, "slow", nil); err != context.DeadlineExceeded {
		t.Fatalf("got %v, want %v", err, context.DeadlineExceeded)
	}

	// The abandoned response must not be handed to the next caller.
	raw, err := b.Scout(context.Background(), "next", nil)
	if err != nil {
		t.Fatalf("Scout after cancel: %v", err)
	}
	var got struct {
		Target string `json:"target"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.Target != "next" {
		t.Fatalf("got a stale response: %+v", got)
	}
}

func TestCallAfterClose(t *testing.T) {
	b, err := sdk.Open(buildModule(t))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := b.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := b.Scout(context.Background(), "x", nil); err != sdk.ErrClosed {
		t.Fatalf("got %v, want %v", err, sdk.ErrClosed)
	}
}
