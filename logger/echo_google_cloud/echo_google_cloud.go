package echogooglecloud

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/global-torque/go-common/configurator"
	"github.com/global-torque/go-common/logger"
	"github.com/rs/zerolog"
)

const (
	errorTypeKey   = "@type"
	errorTypeValue = "type.googleapis.com/google.devtools.clouderrorreporting.v1beta1.ReportedErrorEvent"
)

type EchoGoogleCloud struct {
	skip bool
}

func (h EchoGoogleCloud) Run(e *zerolog.Event, level zerolog.Level, _ string) {
	if h.skip {
		return
	}

	switch level {
	case zerolog.ErrorLevel, zerolog.FatalLevel, zerolog.PanicLevel:
		e.Str(errorTypeKey, errorTypeValue)
	case zerolog.DebugLevel, zerolog.InfoLevel, zerolog.WarnLevel, zerolog.NoLevel, zerolog.Disabled, zerolog.TraceLevel:
	}
}

// NewLogger return logger instance
func NewEchoGCLogger(c context.Context, component string, logLevel string, output io.Writer) logger.Logger {
	level, err := zerolog.ParseLevel(logLevel)
	if err != nil {
		level = zerolog.InfoLevel
	}

	// locally we don't need details for google cloud logging
	_, consoleOutput := output.(zerolog.ConsoleWriter)
	l := zerolog.
		New(output).
		Level(level).
		Hook(logger.SeverityHook{}).
		Hook(logger.ContextHook{}).
		Hook(EchoGoogleCloud{skip: consoleOutput}).
		With().Timestamp()

	// if level == zerolog.DebugLevel || level == zerolog.TraceLevel {
	// l = l.Caller()
	// }

	if component != "" {
		l = l.Str("component", component)
	}

	if c != nil {
		l = l.Ctx(c)
	}

	if err != nil {
		ll := l.Logger()
		ll.Error().Err(err).Interface("level", logLevel).Msg("cannot parse log level, using default info")
	}

	return logger.Logger{Logger: l.Logger()}
}

// DefaultStdoutLogger return default logger instance
func DefaultStdoutLogger(c context.Context, logLevel string) logger.Logger {
	return NewEchoGCLogger(c, "default", logLevel, os.Stdout)
}

// NewComponentLoggerE returns default logger instance with custom component.
func NewComponentLoggerE(c context.Context, component string) (logger.Logger, error) {
	cfg, err := loadConfig()
	log := NewEchoGCLogger(c, component, cfg.LogLevel, outputForConfig(cfg))
	if err != nil {
		log.Error().Err(err).Msg("cannot parse logger config, using defaults")
	}

	return log, err
}

// NewComponentLogger return default logger instance with custom component.
func NewComponentLogger(c context.Context, component string) logger.Logger {
	log, _ := NewComponentLoggerE(c, component)
	return log
}

func loadConfig() (logger.Config, error) {
	cfg := logger.Config{LogLevel: zerolog.InfoLevel.String()}
	if err := configurator.NewConfiguration(&cfg, "logger"); err != nil {
		return cfg, fmt.Errorf("load logger configuration: %w", err)
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = zerolog.InfoLevel.String()
	}

	return cfg, nil
}

func outputForConfig(cfg logger.Config) io.Writer {
	if cfg.LogConsole {
		return zerolog.NewConsoleWriter()
	}

	return os.Stdout
}
