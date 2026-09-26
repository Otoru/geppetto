package logging

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Otoru/geppetto/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewLoggerWritesJSONToStderr(t *testing.T) {
	var stderr bytes.Buffer
	logger, err := NewLogger(LoggerParams{Cfg: &config.Config{LogFormat: "json", LogLevel: "info", Stderr: &stderr}})
	require.NoError(t, err)
	logger.Info("hello", zap.String("key", "value"))
	var entry map[string]any
	require.NoError(t, json.Unmarshal(stderr.Bytes(), &entry))
	assert.Equal(t, "hello", entry["msg"])
	assert.Equal(t, "value", entry["key"])
	assert.Equal(t, "info", entry["level"])
}

func TestNewLoggerDefaultsToInfoLevel(t *testing.T) {
	var stderr bytes.Buffer
	logger, err := NewLogger(LoggerParams{Cfg: &config.Config{LogFormat: "json", LogLevel: "info", Stderr: &stderr}})
	require.NoError(t, err)
	logger.Debug("hidden")
	assert.Empty(t, stderr.String())
}

func TestNewLoggerConsoleFormatInDevelopment(t *testing.T) {
	var stderr bytes.Buffer
	logger, err := NewLogger(LoggerParams{Cfg: &config.Config{LogFormat: "console", LogLevel: "debug", Stderr: &stderr}})
	require.NoError(t, err)
	logger.Debug("hello")
	output := stderr.String()
	assert.Contains(t, output, "hello")
	assert.False(t, json.Valid([]byte(strings.TrimSpace(output))), "console output must not be JSON: %q", output)
}

func TestNewLoggerRejectsInvalidConfig(t *testing.T) {
	_, err := NewLogger(LoggerParams{Cfg: &config.Config{LogFormat: "xml", LogLevel: "info", Stderr: &bytes.Buffer{}}})
	assert.Error(t, err)
	_, err = NewLogger(LoggerParams{Cfg: &config.Config{LogFormat: "json", LogLevel: "loud", Stderr: &bytes.Buffer{}}})
	assert.Error(t, err)
}

func TestNewLoggerSamplingDropsRepeatedEntriesAfterInitial(t *testing.T) {
	var stderr bytes.Buffer
	logger, err := NewLogger(LoggerParams{Cfg: &config.Config{
		LogFormat: "json", LogLevel: "info", Stderr: &stderr,
		LogSampleInterval: time.Hour, LogSampleInitial: 3, LogSampleThereafter: 1000,
	}})
	require.NoError(t, err)
	for range 10 {
		logger.Info("repeated")
	}
	lines := strings.Split(strings.TrimSpace(stderr.String()), "\n")
	assert.Len(t, lines, 3, "sampler must pass only the initial entries of a repeated message")
}

func TestNewLoggerZeroSamplingConfigFallsBackToDefaults(t *testing.T) {
	var stderr bytes.Buffer
	logger, err := NewLogger(LoggerParams{Cfg: &config.Config{LogFormat: "json", LogLevel: "info", Stderr: &stderr}})
	require.NoError(t, err)
	for range DefaultSampleInitial + 1 {
		logger.Info("repeated")
	}
	lines := strings.Split(strings.TrimSpace(stderr.String()), "\n")
	assert.Len(t, lines, DefaultSampleInitial, "zero-value sampling fields must behave like the production defaults")
}
