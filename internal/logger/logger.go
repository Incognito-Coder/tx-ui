package logger

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/op/go-logging"
)

var (
	logger       *logging.Logger
	currentLevel logging.Level = logging.INFO
	logMu        sync.RWMutex
	logBuffer    []struct {
		time  string
		level logging.Level
		log   string
	}
)

func init() {
	InitLogger(logging.INFO)
}

func IsDebug() bool {
	return currentLevel >= logging.DEBUG
}

func InitLogger(level logging.Level) {
	currentLevel = level
	newLogger := logging.MustGetLogger("x-ui")
	var err error
	var backend logging.Backend
	var format logging.Formatter
	ppid := os.Getppid()

	backend, err = logging.NewSyslogBackend("")
	if err != nil {
		backend = logging.NewLogBackend(os.Stderr, "", 0)
	}
	if ppid > 0 && err != nil {
		format = logging.MustStringFormatter(`%{time:2006/01/02 15:04:05} %{level} - %{message}`)
	} else {
		format = logging.MustStringFormatter(`%{level} - %{message}`)
	}

	backendFormatter := logging.NewBackendFormatter(backend, format)
	backendLeveled := logging.AddModuleLevel(backendFormatter)
	backendLeveled.SetLevel(level, "x-ui")
	newLogger.SetBackend(backendLeveled)

	logger = newLogger
}

func Debug(args ...interface{}) {
	if currentLevel < logging.DEBUG {
		return
	}
	logger.Debug(args...)
	addToBuffer(logging.DEBUG, fmt.Sprint(args...))
}

func Debugf(format string, args ...interface{}) {
	if currentLevel < logging.DEBUG {
		return
	}
	logger.Debugf(format, args...)
	addToBuffer(logging.DEBUG, fmt.Sprintf(format, args...))
}

func Info(args ...interface{}) {
	if currentLevel < logging.INFO {
		return
	}
	logger.Info(args...)
	addToBuffer(logging.INFO, fmt.Sprint(args...))
}

func Infof(format string, args ...interface{}) {
	if currentLevel < logging.INFO {
		return
	}
	logger.Infof(format, args...)
	addToBuffer(logging.INFO, fmt.Sprintf(format, args...))
}

func Notice(args ...interface{}) {
	if currentLevel < logging.NOTICE {
		return
	}
	logger.Notice(args...)
	addToBuffer(logging.NOTICE, fmt.Sprint(args...))
}

func Noticef(format string, args ...interface{}) {
	if currentLevel < logging.NOTICE {
		return
	}
	logger.Noticef(format, args...)
	addToBuffer(logging.NOTICE, fmt.Sprintf(format, args...))
}

func Warning(args ...interface{}) {
	if currentLevel < logging.WARNING {
		return
	}
	logger.Warning(args...)
	addToBuffer(logging.WARNING, fmt.Sprint(args...))
}

func Warningf(format string, args ...interface{}) {
	if currentLevel < logging.WARNING {
		return
	}
	logger.Warningf(format, args...)
	addToBuffer(logging.WARNING, fmt.Sprintf(format, args...))
}

func Error(args ...interface{}) {
	if currentLevel < logging.ERROR {
		return
	}
	logger.Error(args...)
	addToBuffer(logging.ERROR, fmt.Sprint(args...))
}

func Errorf(format string, args ...interface{}) {
	if currentLevel < logging.ERROR {
		return
	}
	logger.Errorf(format, args...)
	addToBuffer(logging.ERROR, fmt.Sprintf(format, args...))
}

func addToBuffer(level logging.Level, newLog string) {
	if level > currentLevel {
		return
	}
	t := time.Now()
	logMu.Lock()
	defer logMu.Unlock()
	if len(logBuffer) >= 10240 {
		copy(logBuffer, logBuffer[1:])
		logBuffer = logBuffer[:len(logBuffer)-1]
	}

	logBuffer = append(logBuffer, struct {
		time  string
		level logging.Level
		log   string
	}{
		time:  t.Format("2006/01/02 15:04:05"),
		level: level,
		log:   newLog,
	})
}

func GetLogs(c int, level string) []string {
	var output []string
	logLevel, _ := logging.LogLevel(level)

	logMu.RLock()
	defer logMu.RUnlock()

	for i := len(logBuffer) - 1; i >= 0 && len(output) <= c; i-- {
		if logBuffer[i].level <= logLevel {
			output = append(output, fmt.Sprintf("%s %s - %s", logBuffer[i].time, logBuffer[i].level, logBuffer[i].log))
		}
	}
	return output
}
