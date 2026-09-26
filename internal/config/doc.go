// Package config defines the runtime settings used to wire a geppetto
// process. It is deliberately a small data carrier: command-line parsing,
// profile loading, logging, transport creation, and server construction live
// in their respective packages.
//
// Config includes the selected transport, TCP port, local socket or named
// pipe, profile directory, log format and level, release version, and process
// ID. It also carries the process's stdout and stderr writers so constructors
// use the streams supplied by the caller. Stdout is reserved for the single
// startup handshake line; application logs belong on stderr.
package config
