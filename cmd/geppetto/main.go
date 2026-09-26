package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/spf13/pflag"
	"github.com/vitorhugo/geppetto/internal/config"
	"github.com/vitorhugo/geppetto/internal/logging"
	server "github.com/vitorhugo/geppetto/internal/service"
	"github.com/vitorhugo/geppetto/internal/transport"
	"go.uber.org/fx"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

// developmentVersion is the in-tree fallback stamped into Version; release
// builds replace it via -ldflags "-X main.Version=...".
const developmentVersion = "dev"

// Version is replaced by -ldflags during release builds.
var Version = developmentVersion

const (
	// processName names the binary: it titles the flag set and roots the
	// default per-PID socket path.
	processName = "geppetto"
	// defaultConfigDir is the profile directory used when --config-dir is
	// absent.
	defaultConfigDir = "configs"
)

type handshake struct {
	Transport string `json:"transport"`
	Addr      string `json:"addr"`
	PID       int    `json:"pid"`
	Version   string `json:"version"`
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		logger, _ := zap.NewProduction()
		logger.Error("geppetto stopped", zap.Error(err))
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	cfg, err := parseFlags(args, stderr)
	if err != nil {
		return err
	}
	cfg.Stdout = stdout
	cfg.Stderr = stderr
	cfg.Version = Version
	cfg.PID = os.Getpid()

	serveErr := make(chan error, 1)
	app := fx.New(
		// The client parses exactly one handshake line from stdout before it
		// connects. fx's normal lifecycle logs would corrupt that contract, so
		// fx uses NopLogger and zap writes application logs only to stderr.
		fx.NopLogger,
		fx.Provide(func() *config.Config { return cfg }),
		fx.Provide(logging.NewLogger),
		fx.Provide(server.ProvideProfiles),
		fx.Provide(transport.NewListener),
		fx.Provide(server.NewGRPCServer),
		fx.Invoke(func(lc fx.Lifecycle, log *zap.Logger) {
			lc.Append(fx.Hook{OnStop: func(context.Context) error {
				// Syncing stderr commonly reports a spurious invalid-argument error
				// on normal process shutdown, so it is intentionally not propagated.
				_ = log.Sync()
				return nil
			}})
		}),
		fx.Invoke(func(p lifecycleParams) {
			p.LC.Append(fx.Hook{
				OnStart: func(context.Context) error {
					p.Health.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
					go func() { serveErr <- p.GRPC.Serve(p.Listener) }()
					line := handshake{Transport: p.Listener.Transport, Addr: p.Listener.Addr().String(), PID: cfg.PID, Version: cfg.Version}
					if err := json.NewEncoder(cfg.Stdout).Encode(line); err != nil {
						p.GRPC.Stop()
						return multierr.Append(err, p.Listener.Cleanup())
					}
					p.Log.Info("geppetto listening", zap.String("transport", p.Listener.Transport), zap.String("addr", p.Listener.Addr().String()))
					return nil
				},
				OnStop: func(context.Context) error {
					p.Health.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
					p.GRPC.GracefulStop()
					if err := p.Listener.Cleanup(); err != nil {
						// A SIGTERM-driven shutdown must remain successful. Propagating
						// this error through fx makes the subprocess exit non-zero even
						// though the server has already stopped accepting work.
						p.Log.Warn("listener cleanup failed", zap.Error(err))
					}
					return nil
				},
			})
		}),
	)
	if err := app.Start(ctx); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
	case err := <-serveErr:
		if stopErr := app.Stop(context.Background()); stopErr != nil {
			return stopErr
		}
		return err
	}
	return app.Stop(context.Background())
}

type lifecycleParams struct {
	fx.In
	LC       fx.Lifecycle
	Log      *zap.Logger
	Listener *transport.Listener
	GRPC     *grpc.Server
	Health   *health.Server
}

func parseFlags(args []string, stderr io.Writer) (*config.Config, error) {
	// pflag gives POSIX-style parsing; the flag set below mirrors the previous
	// stdlib flag set exactly — same names, same defaults, ContinueOnError.
	flags := pflag.NewFlagSet(processName, pflag.ContinueOnError)
	flags.SetOutput(stderr)

	transportKind := flags.String("transport", transport.TransportUDS, fmt.Sprintf("%s (Unix socket/named pipe) or %s", transport.TransportUDS, transport.TransportTCP))
	port := flags.Int("port", 0, "TCP port; 0 chooses an ephemeral port")
	socket := flags.String("socket", "", "Unix socket or Windows named pipe path")
	configDir := flags.String("config-dir", defaultConfigDir, "profile JSON directory")
	logFormat := flags.String("log-format", logging.FormatJSON, fmt.Sprintf("log format: %s (production) or %s (development)", logging.FormatJSON, logging.FormatConsole))
	logLevel := flags.String("log-level", zapcore.InfoLevel.String(), "log level: debug, info, warn, error")
	logSampleInterval := flags.Duration("log-sample-interval", logging.DefaultSampleInterval, "log sampling window for identical entries")
	logSampleInitial := flags.Int("log-sample-initial", logging.DefaultSampleInitial, "identical log entries per window that always pass before sampling starts")
	logSampleThereafter := flags.Int("log-sample-thereafter", logging.DefaultSampleThereafter, "after the initial entries, one in N identical entries passes (1 disables sampling)")

	if err := flags.Parse(args); err != nil {
		return nil, err
	}

	if *socket == "" {
		if *transportKind == transport.TransportUDS {
			*socket = defaultSocket(os.Getpid())
		}
	}

	return &config.Config{
		Transport:           *transportKind,
		Port:                *port,
		Socket:              *socket,
		ConfigDir:           *configDir,
		LogFormat:           *logFormat,
		LogLevel:            *logLevel,
		LogSampleInterval:   *logSampleInterval,
		LogSampleInitial:    *logSampleInitial,
		LogSampleThereafter: *logSampleThereafter,
	}, nil
}

func defaultSocket(pid int) string {
	if isWindows() {
		return transport.WindowsPipePrefix + processName + "-" + strconv.Itoa(pid)
	}
	return filepath.Join(os.TempDir(), fmt.Sprintf("%s-%d.sock", processName, pid))
}
