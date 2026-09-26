package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func TestRunEmitsHandshakeServesHealthAndCleansUpSocket(t *testing.T) {
	directory := t.TempDir()
	profile := []byte(`{"name":"test","considerations":[{"id":"ENERGY","value":100,"min":-100,"max":100,"base_weight":1,"critical_threshold":-50,"response_curve":{"kind":"convex","exponent":2}}],"tuning":{"SELECTION_TEMPERATURE":1}}`)
	require.NoError(t, os.WriteFile(filepath.Join(directory, "test.json"), profile, 0o600))
	socket := filepath.Join(os.TempDir(), fmt.Sprintf("npcai-test-%d.sock", os.Getpid()))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stdoutReader, stdoutWriter := io.Pipe()
	defer func() { _ = stdoutReader.Close() }()
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, []string{"--socket", socket, "--config-dir", directory}, stdoutWriter, os.Stderr)
	}()

	var received handshake
	decoded := make(chan error, 1)
	go func() { decoded <- json.NewDecoder(stdoutReader).Decode(&received) }()
	select {
	case err := <-decoded:
		require.NoError(t, err)
	case err := <-done:
		require.NoError(t, err)
		t.Fatal("server exited before emitting a handshake")
	case <-time.After(time.Second):
		t.Fatal("server did not emit a handshake")
	}
	assert.Equal(t, "uds", received.Transport)
	assert.Equal(t, socket, received.Addr)
	assert.NotZero(t, received.PID)

	connection, err := grpc.NewClient("passthrough:///"+socket, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return net.Dial("unix", socket) }))
	require.NoError(t, err)
	defer func() { _ = connection.Close() }()
	healthResponse, err := healthpb.NewHealthClient(connection).Check(context.Background(), &healthpb.HealthCheckRequest{})
	require.NoError(t, err)
	assert.Equal(t, healthpb.HealthCheckResponse_SERVING, healthResponse.Status)

	cancel()
	require.NoError(t, <-done)
	_, err = os.Lstat(socket)
	assert.True(t, os.IsNotExist(err))
}
