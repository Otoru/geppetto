// Package logging builds the process-wide zap logger. All application logs go
// to stderr; stdout is reserved for the handshake line the client parses. It
// does not decide what the application logs.
package logging

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/vitorhugo/npcai/internal/config"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// LoggerParams groups the dependencies of NewLogger. Tests mount it by hand;
// no fx.App is required to call the constructor directly.
type LoggerParams struct {
	fx.In
	Cfg *config.Config
}

// NewLogger builds a *zap.Logger with the typed API. The "json" format is the
// production encoder with sampling; "console" is the development encoder. The
// level defaults to info, keeping debug (and any fine-grained per-batch
// visibility) off unless explicitly requested.
func NewLogger(p LoggerParams) (*zap.Logger, error) {
	level, err := zap.ParseAtomicLevel(p.Cfg.LogLevel)
	if err != nil {
		return nil, fmt.Errorf("invalid log level %q: %w", p.Cfg.LogLevel, err)
	}
	stderr := p.Cfg.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}
	core, err := buildCore(p.Cfg.LogFormat, level, stderr)
	if err != nil {
		return nil, err
	}
	return zap.New(core), nil
}

func buildCore(format string, level zap.AtomicLevel, stderr io.Writer) (zapcore.Core, error) {
	sink := zapcore.Lock(zapcore.AddSync(stderr))
	switch format {
	case "json", "":
		encoderConfig := zap.NewProductionEncoderConfig()
		core := zapcore.NewCore(zapcore.NewJSONEncoder(encoderConfig), sink, level)
		// Sampling matches the zap production defaults: the first 100 entries
		// per second per (level, message, caller) pass, then 1 in 100. This
		// keeps per-batch debug visibility affordable at 50k+ agents per tick.
		return zapcore.NewSamplerWithOptions(core, time.Second, 100, 100), nil
	case "console":
		encoderConfig := zap.NewDevelopmentEncoderConfig()
		return zapcore.NewCore(zapcore.NewConsoleEncoder(encoderConfig), sink, level), nil
	default:
		return nil, fmt.Errorf("invalid log format %q (want json or console)", format)
	}
}
