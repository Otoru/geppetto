package transport

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vitorhugo/npcai/internal/config"
	"go.uber.org/zap"
)

func TestNewListenerFromParams(t *testing.T) {
	listener, err := NewListener(ListenerParams{Cfg: &config.Config{Transport: "tcp", Port: 0}, Log: zap.NewNop()})
	require.NoError(t, err)
	defer func() { _ = listener.Cleanup() }()
	assert.Equal(t, "tcp", listener.Transport)
	assert.NotNil(t, listener.Addr())
}

func TestNewListenerPropagatesListenErrors(t *testing.T) {
	_, err := NewListener(ListenerParams{Cfg: &config.Config{Transport: "carrier-pigeon"}, Log: zap.NewNop()})
	assert.Error(t, err)
}
