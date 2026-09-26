package main

import (
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The command line is a contract: the game client spawns this binary with
// argv. These tests freeze flag names, defaults, and error behavior so the
// parsing library can change without the contract changing.
func TestParseFlagsDefaults(t *testing.T) {
	cfg, err := parseFlags(nil, io.Discard)

	require.NoError(t, err)
	assert.Equal(t, "uds", cfg.Transport)
	assert.Equal(t, 0, cfg.Port)
	assert.Equal(t, "configs", cfg.ConfigDir)
	assert.Equal(t, "json", cfg.LogFormat)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.NotEmpty(t, cfg.Socket, "uds without --socket falls back to a per-PID path")
}

func TestParseFlagsAssignsEveryFlagToItsConfigField(t *testing.T) {
	cfg, err := parseFlags([]string{
		"--transport=tcp",
		"--port=8080",
		"--socket=/tmp/custom.sock",
		"--config-dir=/etc/profiles",
		"--log-format=console",
		"--log-level=debug",
	}, io.Discard)

	require.NoError(t, err)
	assert.Equal(t, "tcp", cfg.Transport)
	assert.Equal(t, 8080, cfg.Port)
	assert.Equal(t, "/tmp/custom.sock", cfg.Socket)
	assert.Equal(t, "/etc/profiles", cfg.ConfigDir)
	assert.Equal(t, "console", cfg.LogFormat)
	assert.Equal(t, "debug", cfg.LogLevel)
}

func TestParseFlagsAcceptsSpaceSeparatedValues(t *testing.T) {
	cfg, err := parseFlags([]string{"--transport", "tcp", "--port", "9090", "--config-dir", "/data"}, io.Discard)

	require.NoError(t, err)
	assert.Equal(t, "tcp", cfg.Transport)
	assert.Equal(t, 9090, cfg.Port)
	assert.Equal(t, "/data", cfg.ConfigDir)
}

func TestParseFlagsLeavesSocketEmptyForTCP(t *testing.T) {
	cfg, err := parseFlags([]string{"--transport=tcp"}, io.Discard)

	require.NoError(t, err)
	assert.Empty(t, cfg.Socket)
}

func TestParseFlagsUnknownFlagFails(t *testing.T) {
	_, err := parseFlags([]string{"--does-not-exist"}, io.Discard)

	require.Error(t, err)
}

func TestParseFlagsInvalidPortFails(t *testing.T) {
	_, err := parseFlags([]string{"--port=not-a-number"}, io.Discard)

	require.Error(t, err)
}

func TestParseFlagsLogSamplingDefaults(t *testing.T) {
	cfg, err := parseFlags(nil, io.Discard)

	require.NoError(t, err)
	assert.Equal(t, time.Second, cfg.LogSampleInterval)
	assert.Equal(t, 100, cfg.LogSampleInitial)
	assert.Equal(t, 100, cfg.LogSampleThereafter)
}

func TestParseFlagsAssignsLogSamplingFlags(t *testing.T) {
	cfg, err := parseFlags([]string{
		"--log-sample-interval=2s",
		"--log-sample-initial=5",
		"--log-sample-thereafter=7",
	}, io.Discard)

	require.NoError(t, err)
	assert.Equal(t, 2*time.Second, cfg.LogSampleInterval)
	assert.Equal(t, 5, cfg.LogSampleInitial)
	assert.Equal(t, 7, cfg.LogSampleThereafter)
}
