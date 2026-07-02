//nolint:gochecknoinits, reassign
package logger

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/pkgerrors"

	"github.com/global-torque/go-common/configurator/v2"
)

func init() {
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
}

// Printf is implementation of fx.Printer
func (l *Logger) Printf(s string, args ...interface{}) {
	l.Info().Msgf(s, args...)
}

// NewLogger return logger instance
func NewLogger(ctx context.Context, component string, logLevel string, output io.Writer) Logger {
	level, err := zerolog.ParseLevel(logLevel)
	if err != nil {
		level = zerolog.InfoLevel
	}

	l := zerolog.
		New(output).
		Level(level).
		Hook(SeverityHook{}).
		Hook(ContextHook{}).
		With().Timestamp()

	if level == zerolog.DebugLevel || level == zerolog.TraceLevel {
		l = l.Caller()
	}

	if ctx != nil {
		l = l.Ctx(ctx)
	}

	if component != "" {
		l = l.Str("component", component)
	}

	if err != nil {
		ll := l.Logger()
		ll.Error().Err(err).Interface("level", logLevel).Msg("cannot parse log level, using default info")
	}

	return Logger{l.Logger()}
}

// DefaultStdoutLogger return default logger instance
func DefaultStdoutLogger(c context.Context, logLevel string) Logger {
	return NewLogger(c, "default", logLevel, os.Stdout)
}

// NewComponentLoggerE returns a default logger instance with a custom component.
func NewComponentLoggerE(c context.Context, component string) (Logger, error) {
	cfg, err := loadConfig("log")
	log := NewLogger(c, component, cfg.LogLevel, outputForConfig(cfg))
	if err != nil {
		log.Error().Err(err).Msg("cannot parse logger config, using defaults")
	}

	return log, err
}

// NewComponentLogger return default logger instance with custom component.
func NewComponentLogger(c context.Context, component string) Logger {
	log, _ := NewComponentLoggerE(c, component)
	return log
}

// FromCtx return default logger instance with custom component
func FromCtx(ctx context.Context, component string) *zerolog.Logger {
	log := zerolog.Ctx(ctx)

	log.UpdateContext(func(c zerolog.Context) zerolog.Context {
		return c.Str("component", component)
	})

	return log
}

// NewDefaultLoggerE returns the default logger instance.
func NewDefaultLoggerE() (Logger, error) {
	cfg, err := loadConfig("log")
	log := NewLogger(context.Background(), "", cfg.LogLevel, outputForConfig(cfg))
	if err != nil {
		log.Error().Err(err).Msg("cannot parse logger config, using defaults")
	}

	return log, err
}

// NewDefaultLogger return default logger instance.
func NewDefaultLogger() Logger {
	log, _ := NewDefaultLoggerE()
	return log
}

func loadConfig(prefix string) (Config, error) {
	cfg := Config{LogLevel: zerolog.InfoLevel.String()}
	if err := configurator.NewConfiguration(&cfg, prefix); err != nil {
		return cfg, fmt.Errorf("load logger configuration: %w", err)
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = zerolog.InfoLevel.String()
	}

	return cfg, nil
}

func outputForConfig(cfg Config) io.Writer {
	if cfg.LogConsole {
		return zerolog.NewConsoleWriter()
	}

	return os.Stdout
}
