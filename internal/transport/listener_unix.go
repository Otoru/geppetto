//go:build !windows

package transport

import (
	"fmt"
	"net"
	"os"
)

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
	listener, err := net.Listen("unix", socket)
	if err != nil {
		return nil, err
	}
	return &Listener{Listener: listener, Transport: "uds", Cleanup: func() error {
		closeErr := listener.Close()
		removeErr := os.Remove(socket)
		if removeErr != nil && !os.IsNotExist(removeErr) {
			return removeErr
		}
		return closeErr
	}}, nil
}
