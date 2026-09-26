// Package logging builds the process-wide zap logger for geppetto. It keeps
// application logs on stderr because stdout is a machine-readable startup
// channel reserved for the handshake JSON.
//
// NewLogger accepts the runtime Config and supports the production JSON
// encoder and the development console encoder. The default level is info;
// debug is opt-in. JSON logging uses zap's production-style sampler so a
// high-volume decision service can expose batch diagnostics without emitting
// one unbounded stream of repeated entries. Invalid levels and formats are
// returned as configuration errors rather than silently changing behavior.
package logging
