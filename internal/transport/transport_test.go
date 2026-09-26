package transport

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListenTCPWithEphemeralPort(t *testing.T) {
	listener, err := Listen("tcp", "", 0)
	require.NoError(t, err)
	defer func() { _ = listener.Cleanup() }()
	assert.Equal(t, "tcp", listener.Transport)
	_, port, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)
	assert.NotEqual(t, "0", port)
}

func TestListenRejectsUnknownTransport(t *testing.T) {
	_, err := Listen("quic", "", 0)
	require.Error(t, err)
}
