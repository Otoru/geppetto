//go:build !windows

package transport

import (
	"fmt"
	"net"
	"os"
)

// unixNetwork is the net.Listen network name for Unix domain sockets; the
// public transport label for the same listener is TransportUDS.
const unixNetwork = "unix"

func listenLocal(socket string) (*Listener, error) {
	if socket == "" {
		return nil, fmt.Errorf("uds socket path is required")
	}
	if info, err := os.Lstat(socket); err == nil {
		if info.Mode()&os.ModeSocket == 0 {
			return nil, fmt.Errorf("refusing to replace non-socket %q", socket)
		}
		if err := os.Remove(socket); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	listener, err := net.Listen(unixNetwork, socket)
	if err != nil {
		return nil, err
	}
	return &Listener{Listener: listener, Transport: TransportUDS, Cleanup: func() error {
		closeErr := listener.Close()
		removeErr := os.Remove(socket)
		if removeErr != nil && !os.IsNotExist(removeErr) {
			return removeErr
		}
		return closeErr
	}}, nil
}
