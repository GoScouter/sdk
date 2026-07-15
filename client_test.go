package sdk

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
)

func withHelperProcess(t *testing.T) {
	t.Helper()
	orig := execCommand
	execCommand = func(_ string, args ...string) *exec.Cmd {
		cs := append([]string{"-test.run=TestHelperProcess", "--"}, args...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
		return cmd
	}
	t.Cleanup(func() { execCommand = orig })
}

type fakeModule struct{}

func (fakeModule) Name() string        { return "fake" }
func (fakeModule) Description() string { return "a fake module" }
func (fakeModule) Version() string     { return "1.2.3" }
func (fakeModule) Scout(target string, args []string) (Result, error) {
	if target == "boom" {
		return nil, fmt.Errorf("kaboom")
	}
	return rawResult("scouted " + target + " flags=" + strings.Join(args, ",")), nil
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	_ = serve(fakeModule{}, os.Stdin, os.Stdout)
	os.Exit(0)
}

func TestOpenReadsDescriptor(t *testing.T) {
	withHelperProcess(t)

	b, err := Open("/path/to/fake-binary")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer b.Close()

	if b.Name() != "fake" {
		t.Errorf("Name() = %q, want fake", b.Name())
	}
	if b.Description() != "a fake module" {
		t.Errorf("Description() = %q", b.Description())
	}
	if b.Version() != "1.2.3" {
		t.Errorf("Version() = %q", b.Version())
	}
	if b.Path() != "/path/to/fake-binary" {
		t.Errorf("Path() = %q", b.Path())
	}
}

func TestBinaryScoutRoundTrip(t *testing.T) {
	withHelperProcess(t)

	b, err := Open("/path/to/fake-binary")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer b.Close()

	res, err := b.Scout("example.com", []string{"--https"})
	if err != nil {
		t.Fatalf("Scout: %v", err)
	}
	got := res.Render()
	if !strings.Contains(got, "scouted example.com") {
		t.Errorf("Render() = %q", got)
	}
	if !strings.Contains(got, "flags=--https") {
		t.Errorf("Render() = %q, want forwarded --https flag", got)
	}
}

func TestBinaryReusedAcrossScouts(t *testing.T) {
	withHelperProcess(t)

	b, err := Open("/path/to/fake-binary")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer b.Close()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			target := fmt.Sprintf("host%d.example.com", i)
			res, err := b.Scout(target, nil)
			if err != nil {
				t.Errorf("Scout(%s): %v", target, err)
				return
			}
			if !strings.Contains(res.Render(), "scouted "+target) {
				t.Errorf("Scout(%s) = %q", target, res.Render())
			}
		}(i)
	}
	wg.Wait()
}

func TestScoutErrorPropagates(t *testing.T) {
	withHelperProcess(t)

	b, err := Open("/path/to/fake-binary")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer b.Close()

	if _, err := b.Scout("boom", nil); err == nil {
		t.Fatal("Scout of failing target should return an error")
	} else if !strings.Contains(err.Error(), "kaboom") {
		t.Errorf("error = %v, want it to mention kaboom", err)
	}
}

func TestScoutAfterCloseFails(t *testing.T) {
	withHelperProcess(t)

	b, err := Open("/path/to/fake-binary")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	b.Close()

	if _, err := b.Scout("example.com", nil); err == nil {
		t.Fatal("Scout after Close should fail")
	}
}
