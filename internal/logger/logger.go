package logger

import (
	"os"
	"sync"
	"time"

	"github.com/harry-urek/urek/v/internal/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	log  *zap.Logger
	once sync.Once
)

func InitLogger(cfg *config.LoggerConfig) *zap.Logger {
	once.Do(func() {
		// encoder Config
		encoderCfg := zapcore.EncoderConfig{
			TimeKey:        "ts",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			FunctionKey:    zapcore.OmitKey,
			MessageKey:     "msg",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.StringDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		}

		// Configure the logger
		var encoder zapcore.Encoder
		if cfg.Format == "json" {
			encoder = zapcore.NewJSONEncoder(encoderCfg)
		} else {
			encoder = zapcore.NewConsoleEncoder(encoderCfg)
		}

		// Create the logger
		var level zapcore.Level
		switch cfg.Level {
		case "debug":
			level = zapcore.DebugLevel
		case "info":
			level = zapcore.InfoLevel
		case "warn":
			level = zapcore.WarnLevel
		case "error":
			level = zapcore.ErrorLevel
		default:
			level = zapcore.InfoLevel
		}

		// Configure output
		var output zapcore.WriteSyncer
		switch cfg.OutputPath {
		case "stdout":
			output = zapcore.AddSync(os.Stdout)
		default:
			// 0644 -> read/write for owner, read for group and others
			file, err := os.OpenFile(cfg.OutputPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				file = os.Stdout
			}
			output = zapcore.AddSync(file)
		}

		core := zapcore.NewCore(encoder, output, zap.NewAtomicLevelAt(level))
		log = zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
	})

	return log
}

func GetLogger() *zap.Logger {
	if log == nil {
		// default logger
		return zap.NewExample()
	}
	return log
}

// Named returns a named logger
func Named(name string) *zap.Logger {
	return GetLogger().Named(name)
}

// logger with given fields
func With(fields ...zapcore.Field) *zap.Logger {
	return GetLogger().With(fields...)
}

func SetLevel(level string) {
	var zapLevel zapcore.Level
	switch level {
	case "debug":
		zapLevel = zapcore.DebugLevel
	case "info":
		zapLevel = zapcore.InfoLevel
	case "warn":
		zapLevel = zapcore.WarnLevel
	case "error":
		zapLevel = zapcore.ErrorLevel
	default:
		zapLevel = zapcore.InfoLevel
	}

	if log != nil {
		log.Core().Enabled(zapLevel)
	}
}

func LogHTTPRequest(method, path, ip string, statusCode int, latency time.Duration) {
	GetLogger().Info("HTTP Request",
		zap.String("method", method),
		zap.String("path", path),
		zap.String("ip", ip),
		zap.Int("status", statusCode),
		zap.Duration("latency", latency),
	)
}

// LogGRPCRequest
func LogGRPCRequest(method string, latency time.Duration, err error) {
	logger := GetLogger()
	if err != nil {
		logger.Warn("gRPC Request Failed",
			zap.String("method", method),
			zap.Duration("latency", latency),
			zap.Error(err),
		)
	} else {
		logger.Info("gRPC Request",
			zap.String("method", method),
			zap.Duration("latency", latency),
		)
	}
}
