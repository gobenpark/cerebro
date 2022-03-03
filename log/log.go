package log

import (
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type zapLogger struct {
	Logger *zap.Logger
}

func NewZapLogger() Logger {
	//logger, err := zap.Config{
	//	Level:             zap.NewAtomicLevelAt(zapcore.InfoLevel),
	//	Development:       false,
	//	DisableCaller:     false,
	//	DisableStacktrace: false,
	//	Sampling: &zap.SamplingConfig{
	//		Initial:    100,
	//		Thereafter: 100,
	//	},
	//	Encoding: "json",
	//	EncoderConfig: zapcore.EncoderConfig{
	//		TimeKey:        "eventTime",
	//		LevelKey:       "severity",
	//		NameKey:        "logger",
	//		CallerKey:      "caller",
	//		MessageKey:     "message",
	//		StacktraceKey:  "stacktrace",
	//		LineEnding:     zapcore.DefaultLineEnding,
	//		EncodeTime:     zapcore.ISO8601TimeEncoder,
	//		EncodeDuration: zapcore.SecondsDurationEncoder,
	//		EncodeCaller:   zapcore.ShortCallerEncoder,
	//	},
	//	OutputPaths:      []string{"stdout"},
	//	ErrorOutputPaths: []string{"stderr"},
	//	InitialFields:    nil,
	//}.Build(zap.AddCallerSkip(1))
	config := zap.NewProductionConfig()
	config.Level = zap.NewAtomicLevelAt(zapcore.WarnLevel)
	logger, err := config.Build(zap.AddCallerSkip(1))
	if err != nil {
		panic(err)
	}

	return zapLogger{logger}
}

func (z zapLogger) With(key string, value interface{}) Logger {
	return zapLogger{
		Logger: z.Logger.With(zap.Any(key, value)),
	}
}

func (z zapLogger) Debug(v ...interface{}) {
	defer z.Logger.Sync()
	z.Logger.Debug("debug")
}

func (z zapLogger) Debugf(format string, v ...interface{}) {
	defer z.Logger.Sync()
	z.Logger.Debug(fmt.Sprintf(format, v...))
}

func (z zapLogger) Error(v ...interface{}) {
	defer z.Logger.Sync()
	z.Logger.Error(fmt.Sprint(v...))
}

func (z zapLogger) Errorf(format string, v ...interface{}) {
	defer z.Logger.Sync()
	z.Logger.Error(fmt.Sprintf(format, v...))
}

func (z zapLogger) Info(v ...interface{}) {
	defer z.Logger.Sync()
	z.Logger.Info(fmt.Sprint(v...))
}

func (z zapLogger) Infof(format string, v ...interface{}) {
	defer z.Logger.Sync()
	z.Logger.Info(fmt.Sprintf(format, v...))
}

func (z zapLogger) Warning(v ...interface{}) {
	defer z.Logger.Sync()
	z.Logger.Warn(fmt.Sprint(v...))
}

func (z zapLogger) Warningf(format string, v ...interface{}) {
	defer z.Logger.Sync()
	z.Logger.Warn(fmt.Sprintf(format, v...))
}

func (z zapLogger) Fatal(v ...interface{}) {
	defer z.Logger.Sync()
	z.Logger.Fatal(fmt.Sprint(v...))
}

func (z zapLogger) Fatalf(format string, v ...interface{}) {
	defer z.Logger.Sync()
	z.Logger.Fatal(fmt.Sprintf(format, v...))
}

func (z zapLogger) Panic(v ...interface{}) {
	defer z.Logger.Sync()
	z.Logger.Panic(fmt.Sprint(v...))
}

func (z zapLogger) Panicf(format string, v ...interface{}) {
	defer z.Logger.Sync()
	z.Logger.Panic(fmt.Sprintf(format, v...))
}

func (z zapLogger) Clone(name string) Logger {
	return zapLogger{z.Logger.With(zap.String("name", name))}
}
