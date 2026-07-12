package sdk

import (
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"strings"
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

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	args := os.Args
	for i, a := range args {
		if a == "--" {
			args = args[i+1:]
			break
		}
	}
	if len(args) == 0 {
		os.Exit(2)
	}

	switch args[0] {
	case cmdDescribe:
		_ = json.NewEncoder(os.Stdout).Encode(Descriptor{
			Protocol:    ProtocolVersion,
			Name:        "fake",
			Description: "a fake module",
			Version:     "1.2.3",
		})
		os.Exit(0)
	case cmdScout:
		var target string
		for i := 1; i+1 < len(args); i++ {
			if args[i] == "-target" {
				target = args[i+1]
			}
		}
		io.WriteString(os.Stdout, "scouted "+target)
		os.Exit(0)
	default:
		os.Exit(2)
	}
}

func TestOpenReadsDescriptor(t *testing.T) {
	withHelperProcess(t)

	b, err := Open("/path/to/fake-binary")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
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

	res, err := b.Scout("example.com")
	if err != nil {
		t.Fatalf("Scout: %v", err)
	}
	if got := res.Render(); !strings.Contains(got, "scouted example.com") {
		t.Errorf("Render() = %q", got)
	}
}
