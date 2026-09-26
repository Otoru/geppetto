//go:build !windows

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// lockedBuffer lets the test poll the child process stdout while the exec
// copier goroutine is still writing to it.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// TestBinaryStdoutContainsOnlyTheHandshakeLine proves the stdout contract: the
// client parses exactly one JSON handshake line, so no log byte may ever reach
// stdout. It also proves the process shuts down cleanly on SIGTERM.
func TestBinaryStdoutContainsOnlyTheHandshakeLine(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "geppetto")
	buildOutput, err := exec.Command("go", "build", "-o", binary, ".").CombinedOutput()
	require.NoError(t, err, string(buildOutput))

	directory := t.TempDir()
	profile := []byte(`{"name":"test","considerations":[{"id":"ENERGY","value":100,"min":-100,"max":100,"base_weight":1,"critical_threshold":-50,"response_curve":{"kind":"convex","exponent":2}}],"tuning":{"SELECTION_TEMPERATURE":1}}`)
	require.NoError(t, os.WriteFile(filepath.Join(directory, "test.json"), profile, 0o600))
	socket := filepath.Join(os.TempDir(), fmt.Sprintf("geppetto-stdout-contract-%d.sock", os.Getpid()))

	cmd := exec.Command(binary, "--socket", socket, "--config-dir", directory)
	var stdout, stderr lockedBuffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	require.NoError(t, cmd.Start())
	defer func() { _ = cmd.Process.Kill() }()

	// The handshake line is the readiness signal: the socket may accept
	// connections before it is written, so poll stdout instead.
	deadline := time.Now().Add(10 * time.Second)
	for !strings.Contains(stdout.String(), "\n") {
		if time.Now().After(deadline) {
			t.Fatalf("server did not emit a handshake; stderr: %s", stderr.String())
		}
		time.Sleep(10 * time.Millisecond)
	}

	require.NoError(t, cmd.Process.Signal(syscall.SIGTERM))
	require.NoError(t, cmd.Wait(), "process must exit cleanly on SIGTERM; stderr: %s", stderr.String())

	content := stdout.String()
	require.True(t, strings.HasSuffix(content, "\n"), "handshake line must end with a newline, got %q", content)
	lines := strings.Split(strings.TrimSuffix(content, "\n"), "\n")
	require.Len(t, lines, 1, "stdout must contain exactly the handshake line, got %q", content)
	var received handshake
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &received))
	assert.Equal(t, "uds", received.Transport)
	assert.Equal(t, socket, received.Addr)
	assert.NotZero(t, received.PID)
	assert.NotEmpty(t, received.Version)
}
