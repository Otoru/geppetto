package logging

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/vitorhugo/geppetto/internal/config"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Log formats accepted by NewLogger. The command-line default, the flag help
// text, and the encoder switch all reuse these constants so the vocabulary
// lives in exactly one place.
const (
	FormatJSON    = "json"
	FormatConsole = "console"
)

// Production sampling defaults, matching zap's production configuration: the
// first DefaultSampleInitial entries per DefaultSampleInterval per
// (level, message, caller) pass, then one in DefaultSampleThereafter. This
// keeps per-batch debug visibility affordable at 50k+ agents per tick.
const (
	DefaultSampleInterval   = time.Second
	DefaultSampleInitial    = 100
	DefaultSampleThereafter = 100
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
	core, err := buildCore(p.Cfg.LogFormat, level, stderr, samplerConfigFor(p.Cfg))
	if err != nil {
		return nil, err
	}
	return zap.New(core), nil
}

// samplerConfig holds the resolved parameters of the production sampler.
type samplerConfig struct {
	interval   time.Duration
	initial    int
	thereafter int
}

// samplerConfigFor resolves the configured sampler parameters, falling back
// to the production defaults for zero-value fields so a hand-built Config
// keeps the default behavior. There is no "disable" value on purpose: the
// sampler only drops repeated identical entries, and thereafter=1 already
// lets everything through.
func samplerConfigFor(cfg *config.Config) samplerConfig {
	resolved := samplerConfig{
		interval:   cfg.LogSampleInterval,
		initial:    cfg.LogSampleInitial,
		thereafter: cfg.LogSampleThereafter,
	}
	if resolved.interval <= 0 {
		resolved.interval = DefaultSampleInterval
	}
	if resolved.initial <= 0 {
		resolved.initial = DefaultSampleInitial
	}
	if resolved.thereafter <= 0 {
		resolved.thereafter = DefaultSampleThereafter
	}
	return resolved
}

func buildCore(format string, level zap.AtomicLevel, stderr io.Writer, sampler samplerConfig) (zapcore.Core, error) {
	sink := zapcore.Lock(zapcore.AddSync(stderr))
	switch format {
	case FormatJSON, "":
		encoderConfig := zap.NewProductionEncoderConfig()
		core := zapcore.NewCore(zapcore.NewJSONEncoder(encoderConfig), sink, level)
		return zapcore.NewSamplerWithOptions(core, sampler.interval, sampler.initial, sampler.thereafter), nil
	case FormatConsole:
		encoderConfig := zap.NewDevelopmentEncoderConfig()
		return zapcore.NewCore(zapcore.NewConsoleEncoder(encoderConfig), sink, level), nil
	default:
		return nil, fmt.Errorf("invalid log format %q (want %s or %s)", format, FormatJSON, FormatConsole)
	}
}
