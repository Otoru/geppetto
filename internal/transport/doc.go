// Package transport creates the process listener selected by the command-line
// configuration. It owns the platform-specific listener and cleanup handle,
// but it does not register gRPC services or manage the gRPC server lifecycle.
//
// The default local transport is a Unix domain socket on macOS and Linux. On
// Windows, the same "uds" configuration name selects a named pipe with the
// required \\.\pipe\ prefix. TCP is also supported for development and binds
// to 127.0.0.1; port zero asks the operating system for an ephemeral port.
// The listener records the transport label and the address that the command
// publishes in its startup handshake.
//
// A normal process uses a per-PID local socket or named pipe, so a fresh
// process has a fresh address. Development hot reload can instead use a fixed
// path such as tmp/geppetto-dev.sock, or a fixed TCP port. A stable address
// means the client does not have to discover a new endpoint after every Air
// rebuild, but it does not preserve the connection: the old process exits,
// the old connection dies, and the client still has to reconnect. The same
// limitation applies whether the stable endpoint is a Unix socket, named pipe,
// or fixed TCP port.
//
// Unix cleanup closes the listener and removes the socket path. Windows
// cleanup closes the named pipe, and TCP cleanup closes the network listener.
// Local socket creation refuses to replace an existing non-socket path, which
// prevents a stale or misconfigured path from being silently overwritten.
package transport
