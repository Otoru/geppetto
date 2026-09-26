package config

import (
	"io"
	"time"
)

// Config is the single source of truth for process wiring. Stdout and Stderr
// are carried here so every constructor writes to the streams the process was
// started with; stdout is reserved for the handshake line, logs go to stderr.
//
// The LogSample* fields tune the production log sampler. A zero value means
// "not configured" and falls back to the defaults in internal/logging, so a
// hand-built Config keeps the default behavior.
type Config struct {
	Transport           string
	Port                int
	Socket              string
	ConfigDir           string
	LogFormat           string
	LogLevel            string
	LogSampleInterval   time.Duration
	LogSampleInitial    int
	LogSampleThereafter int
	Version             string
	PID                 int
	Stdout              io.Writer
	Stderr              io.Writer
}
