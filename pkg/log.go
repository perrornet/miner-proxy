package pkg

import (
	"io"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	_log *zap.SugaredLogger
)

type GormLog struct {
	l *zap.SugaredLogger
}

func InitLog(level zapcore.Level, logfile string, syncs ...io.Writer) {
	encoderConfig := &zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		MessageKey:     "message",
		StacktraceKey:  "trace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05"),
		EncodeDuration: zapcore.MillisDurationEncoder,
	}

	atomicLevel := zap.NewAtomicLevelAt(level)
	var ws []zapcore.WriteSyncer

	ws = append(ws, zapcore.AddSync(os.Stdout))
	if logfile != "" {
		hook := &lumberjack.Logger{
			Filename:   logfile, // 日志文件路径
			MaxSize:    100,
			MaxBackups: 5,
			MaxAge:     7,
			Compress:   false,
		}
		ws = append(ws, zapcore.AddSync(hook))
	}
	for _, v := range syncs {
		ws = append(ws, zapcore.AddSync(v))
	}

	core := zapcore.NewCore(zapcore.NewJSONEncoder(*encoderConfig),
		zapcore.NewMultiWriteSyncer(ws...), atomicLevel)

	var options = []zap.Option{zap.AddCallerSkip(2)}
	if level <= zap.DebugLevel {
		options = append(options, zap.AddCaller())
	}
	_log = zap.New(core, options...).Sugar()
}

func outByLog(level zapcore.Level, msg string, v ...interface{}) {
	if _log == nil {
		InitLog(zap.DebugLevel, "")
		_log.Warn("log not init, auto init log level debug")
	}
	var out = _log.Debugf

	switch level {
	case zapcore.DebugLevel:
		out = _log.Debugf
	case zapcore.InfoLevel:
		out = _log.Infof
	case zapcore.WarnLevel:
		out = _log.Warnf
	case zapcore.ErrorLevel:
		out = _log.Errorf
	case zapcore.FatalLevel:
		out = _log.Fatalf
	case zapcore.PanicLevel:
		out = _log.Panicf
	case zapcore.DPanicLevel:
		out = _log.DPanicf
	}
	out(msg, v...)
}

func Debug(msg string, v ...interface{}) {
	outByLog(zapcore.DebugLevel, msg, v...)
}

func Info(msg string, v ...interface{}) {
	outByLog(zapcore.InfoLevel, msg, v...)
}

func Warn(msg string, v ...interface{}) {
	outByLog(zapcore.WarnLevel, msg, v...)
}

func Error(msg string, v ...interface{}) {
	outByLog(zapcore.ErrorLevel, msg, v...)
}

func Panic(msg string, v ...interface{}) {
	outByLog(zapcore.PanicLevel, msg, v...)
}

func Fatal(msg string, v ...interface{}) {
	outByLog(zapcore.FatalLevel, msg, v...)
}
