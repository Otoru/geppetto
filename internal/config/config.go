// Package config carries runtime settings resolved from CLI flags. It does not
// parse flags or construct runtime dependencies.
package config

import "io"

// Config is the single source of truth for process wiring. Stdout and Stderr
// are carried here so every constructor writes to the streams the process was
// started with; stdout is reserved for the handshake line, logs go to stderr.
type Config struct {
	Transport string
	Port      int
	Socket    string
	ConfigDir string
	LogFormat string
	LogLevel  string
	Version   string
	PID       int
	Stdout    io.Writer
	Stderr    io.Writer
}
