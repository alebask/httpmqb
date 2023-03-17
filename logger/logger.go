package logger

import (
	"os"

	"github.com/sirupsen/logrus"
)

var appLog = New()

type Fields map[string]interface{}

type Logger struct {
	log *logrus.Logger
}

func (l *Logger) Info(msg string, fields Fields) {
	l.log.WithFields(logrus.Fields(fields)).Info(msg)
}
func (l *Logger) Warning(msg string, fields Fields) {
	l.log.WithFields(logrus.Fields(fields)).Warn(msg)
}
func (l *Logger) Error(msg string, fields Fields) {
	l.log.WithFields(logrus.Fields(fields)).Error(msg)
}

func Info(msg string, fields Fields) {
	appLog.Info(msg, fields)
}
func Warning(msg string, fields Fields) {
	appLog.Warning(msg, fields)
}
func Error(msg string, fields Fields) {
	appLog.Error(msg, fields)
}

func New() *Logger {

	lgrus := logrus.New()

	lgrus.SetOutput(os.Stdout)
	lgrus.SetFormatter(&logrus.JSONFormatter{})
	lgrus.SetLevel(logrus.InfoLevel)

	return &Logger{log: lgrus}
}
