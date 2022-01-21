package pkg

import (
	"io"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	_log    *zap.Logger
	logFunc map[zapcore.Level]func(template string, args ...interface{})
)

type GormLog struct {
	l *zap.SugaredLogger
}

func initOutFunc() {
	if len(logFunc) == 0 {
		logFunc = map[zapcore.Level]func(template string, args ...interface{}){
			zapcore.DebugLevel:  _log.Sugar().Debugf,
			zapcore.InfoLevel:   _log.Sugar().Infof,
			zapcore.WarnLevel:   _log.Sugar().Warnf,
			zapcore.ErrorLevel:  _log.Sugar().Errorf,
			zapcore.FatalLevel:  _log.Sugar().Fatalf,
			zapcore.PanicLevel:  _log.Sugar().Panicf,
			zapcore.DPanicLevel: _log.Sugar().DPanicf,
		}
	}
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
	_log = zap.New(core, options...)
	initOutFunc()
}

func outByLog(level zapcore.Level, msg string, v ...interface{}) {
	if _log == nil {
		InitLog(zap.DebugLevel, "")
		initOutFunc()
		_log.Warn("log not init, auto init log level debug")
	}

	var out = _log.Sugar().Debugf
	if _, ok := logFunc[level]; ok {
		out = logFunc[level]
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

func DPanic(msg string, v ...interface{}) {
	outByLog(zapcore.DPanicLevel, msg, v...)
}

func Fatal(msg string, v ...interface{}) {
	outByLog(zapcore.FatalLevel, msg, v...)
}

func Log() *zap.Logger {
	return _log
}
