package log

import (
	"os"
	"strconv"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var ZapLogger *zap.Logger

func init() {
	pe := zap.NewProductionEncoderConfig()
	pe.EncodeTime = zapcore.ISO8601TimeEncoder // The encoder can be customized for each output
	pe.TimeKey = "time"
	pe.CallerKey = "context"
	// set duration field unit to millisecond
	pe.EncodeDuration = zapcore.MillisDurationEncoder
	consoleEncoder := zapcore.NewJSONEncoder(pe)
	// default InfoLevel
	level := zapcore.Level(getLoggerLevel())
	core := zapcore.NewTee(
		// log to console
		zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), level),
	)
	logger := zap.New(core, zap.AddCaller()).With(zap.Namespace("attr"))
	ZapLogger = logger
}

func getLoggerLevel() int8 {
	loggerLevel := os.Getenv("LOGGER_LEVEL")
	if loggerLevel == "debug" {
		return -1
	} else if loggerLevel == "info" {
		return 0
	} else if l, err := strconv.Atoi(loggerLevel); err == nil {
		return int8(l)
	} else {
		return 0
	}
}
