// Package transport creates the process listener selected by the CLI. It does
// not register gRPC services or manage their lifecycle.
package transport

import (
	"fmt"
	"net"
	"strconv"

	"github.com/vitorhugo/geppetto/internal/config"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// ListenerParams groups the dependencies of NewListener. Tests mount it by
// hand; no fx.App is required to call the constructor directly.
type ListenerParams struct {
	fx.In
	Cfg *config.Config
	Log *zap.Logger
}

// NewListener resolves the listener from the process config.
func NewListener(p ListenerParams) (*Listener, error) {
	listener, err := Listen(p.Cfg.Transport, p.Cfg.Socket, p.Cfg.Port)
	if err != nil {
		return nil, err
	}
	p.Log.Debug("listener created", zap.String("transport", listener.Transport), zap.String("addr", listener.Addr().String()))
	return listener, nil
}

// Listener owns a listener and any platform-specific cleanup action.
type Listener struct {
	net.Listener
	Transport string
	Cleanup   func() error
}

func Listen(kind, socket string, port int) (*Listener, error) {
	switch kind {
	case "tcp":
		listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
		if err != nil {
			return nil, err
		}
		return &Listener{Listener: listener, Transport: "tcp", Cleanup: listener.Close}, nil
	case "uds":
		return listenLocal(socket)
	default:
		return nil, fmt.Errorf("unsupported transport %q (want uds or tcp)", kind)
	}
}
