package sdk

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// execCommand is indirected so tests can substitute a fake subprocess.
var execCommand = exec.Command

// Binary is a handle to a downloaded module binary. It implements [Module] by
// invoking the binary as a subprocess, so the host can treat a downloaded
// module exactly like an in-process one.
//
// Downloading, verifying, and caching the binary on disk is the host's
// responsibility; Binary only speaks the protocol to an already-present
// executable at path.
type Binary struct {
	path string
	desc Descriptor
}

// Open loads the module binary at path. It runs the binary's "describe" command
// to read its metadata and rejects binaries built against an incompatible
// [ProtocolVersion].
func Open(path string) (*Binary, error) {
	var stdout, stderr bytes.Buffer
	cmd := execCommand(path, cmdDescribe)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("sdk: describe %s: %w: %s", path, err, strings.TrimSpace(stderr.String()))
	}

	var d Descriptor
	if err := json.Unmarshal(stdout.Bytes(), &d); err != nil {
		return nil, fmt.Errorf("sdk: decoding descriptor from %s: %w", path, err)
	}
	if d.Protocol != ProtocolVersion {
		return nil, fmt.Errorf("sdk: %s uses protocol %d, host requires %d", path, d.Protocol, ProtocolVersion)
	}
	if d.Name == "" {
		return nil, fmt.Errorf("sdk: %s reported an empty module name", path)
	}

	return &Binary{path: path, desc: d}, nil
}

// Path returns the filesystem path of the underlying binary.
func (b *Binary) Path() string { return b.path }

func (b *Binary) Name() string        { return b.desc.Name }
func (b *Binary) Description() string { return b.desc.Description }
func (b *Binary) Version() string     { return b.desc.Version }

// Scout runs the binary's "scout" command against target and returns its
// rendered output as a [Result].
func (b *Binary) Scout(target string) (Result, error) {
	var stdout, stderr bytes.Buffer
	cmd := execCommand(b.path, cmdScout, "-target", target)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("sdk: scout %q via %s: %s", target, b.desc.Name, msg)
	}

	return rawResult(stdout.String()), nil
}

// rawResult is a Result whose rendering is the text produced by a module binary.
type rawResult string

func (r rawResult) Render() string { return string(r) }
