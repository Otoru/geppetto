package transport

import (
	"fmt"
	"net"
	"strconv"

	"github.com/Otoru/geppetto/internal/config"
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

// Transport vocabulary shared by the CLI default, the flag help text, the
// listener switch, and the handshake's transport label: one place only.
const (
	TransportTCP = "tcp"
	TransportUDS = "uds"
)

// loopbackHost binds TCP to loopback only: the subprocess serves the local
// game client and must not accept remote connections.
const loopbackHost = "127.0.0.1"

// WindowsPipePrefix is the mandatory path prefix for Windows named pipes. It
// lives here (not behind the Windows build tag) because the command's default
// socket path also needs it when running on Windows.
const WindowsPipePrefix = `\\.\pipe\`

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
	case TransportTCP:
		listener, err := net.Listen(TransportTCP, net.JoinHostPort(loopbackHost, strconv.Itoa(port)))
		if err != nil {
			return nil, err
		}
		return &Listener{Listener: listener, Transport: TransportTCP, Cleanup: listener.Close}, nil
	case TransportUDS:
		return listenLocal(socket)
	default:
		return nil, fmt.Errorf("unsupported transport %q (want %s or %s)", kind, TransportUDS, TransportTCP)
	}
}
