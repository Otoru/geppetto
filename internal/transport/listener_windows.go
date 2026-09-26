//go:build windows

package transport

import (
	"fmt"
	"strings"

	"github.com/Microsoft/go-winio"
)

func listenLocal(pipe string) (*Listener, error) {
	if pipe == "" {
		return nil, fmt.Errorf("named pipe path is required")
	}
	if !strings.HasPrefix(pipe, `\\.\pipe\`) {
		return nil, fmt.Errorf("named pipe must start with \\.\\pipe\\")
	}
	listener, err := winio.ListenPipe(pipe, nil)
	if err != nil {
		return nil, err
	}
	return &Listener{Listener: listener, Transport: "uds", Cleanup: listener.Close}, nil
}
