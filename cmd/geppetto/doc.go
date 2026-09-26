// Package main is the geppetto subprocess entry point. It wires the runtime
// configuration, profile cache, logger, listener, gRPC decision service, and
// health service with fx, then keeps the process alive until its context is
// cancelled or serving fails. The command is stateless with respect to NPCs:
// all decision state arrives in each RPC, and engine scoring remains in
// internal/engine.
//
// # Invocation contract
//
// The first and only startup line written to stdout is one JSON handshake. It
// contains transport, addr, pid, and version. A game client must read and
// parse that line before it attempts to connect. Stdout is therefore a wire
// contract, not a general-purpose log sink; all application logs are sent to
// stderr by internal/logging. fx is configured with NopLogger so framework
// lifecycle messages cannot corrupt the handshake.
//
// The command accepts these flags and defaults:
//
//   - --transport=uds selects a Unix socket on Unix or a named pipe on Windows;
//     --transport=tcp selects loopback TCP.
//   - --port=0 asks the operating system for an ephemeral TCP port.
//   - --socket= is empty by default and becomes a per-PID path for local
//     transport; an explicit value is used unchanged.
//   - --config-dir=configs selects the JSON profile directory.
//   - --log-format=json selects production JSON; console is the development
//     alternative.
//   - --log-level=info selects the default log threshold; debug, warn, and
//     error are also accepted by zap.
//
// The default local address is intentionally process-specific, so the client
// learns it from the handshake rather than guessing. Development hot reload
// can pass a stable socket path (or a fixed TCP port), but a stable address
// does not keep a connection alive across reload: the client still has to
// reconnect after the old subprocess exits.
//
// # Lifecycle
//
// run parses flags, attaches the caller's stdout and stderr writers, loads
// profiles, creates the listener and gRPC services, and starts the fx
// application. On start it marks the gRPC health service SERVING, begins the
// server, and emits the handshake. On shutdown it marks the service
// NOT_SERVING, performs gRPC GracefulStop, and cleans up the listener. SIGINT
// and SIGTERM cancel the root context and follow this graceful path. Logger
// sync errors on ordinary stderr shutdown and listener cleanup warnings after
// the server has stopped are deliberately not allowed to turn a successful
// signal-driven shutdown into a false non-zero exit.
package main
