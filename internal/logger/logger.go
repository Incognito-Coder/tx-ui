package logger

import (
	"fmt"
	"os"
	"strings"
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
	addToBuffer(logging.DEBUG, fmt.Sprint(args...))
	if currentLevel < logging.DEBUG {
		return
	}
	logger.Debug(args...)
}

func Debugf(format string, args ...interface{}) {
	addToBuffer(logging.DEBUG, fmt.Sprintf(format, args...))
	if currentLevel < logging.DEBUG {
		return
	}
	logger.Debugf(format, args...)
}

func Info(args ...interface{}) {
	addToBuffer(logging.INFO, fmt.Sprint(args...))
	if currentLevel < logging.INFO {
		return
	}
	logger.Info(args...)
}

func Infof(format string, args ...interface{}) {
	addToBuffer(logging.INFO, fmt.Sprintf(format, args...))
	if currentLevel < logging.INFO {
		return
	}
	logger.Infof(format, args...)
}

func Notice(args ...interface{}) {
	addToBuffer(logging.NOTICE, fmt.Sprint(args...))
	if currentLevel < logging.NOTICE {
		return
	}
	logger.Notice(args...)
}

func Noticef(format string, args ...interface{}) {
	addToBuffer(logging.NOTICE, fmt.Sprintf(format, args...))
	if currentLevel < logging.NOTICE {
		return
	}
	logger.Noticef(format, args...)
}

func Warning(args ...interface{}) {
	addToBuffer(logging.WARNING, fmt.Sprint(args...))
	if currentLevel < logging.WARNING {
		return
	}
	logger.Warning(args...)
}

func Warningf(format string, args ...interface{}) {
	addToBuffer(logging.WARNING, fmt.Sprintf(format, args...))
	if currentLevel < logging.WARNING {
		return
	}
	logger.Warningf(format, args...)
}

func Error(args ...interface{}) {
	addToBuffer(logging.ERROR, fmt.Sprint(args...))
	if currentLevel < logging.ERROR {
		return
	}
	logger.Error(args...)
}

func Errorf(format string, args ...interface{}) {
	addToBuffer(logging.ERROR, fmt.Sprintf(format, args...))
	if currentLevel < logging.ERROR {
		return
	}
	logger.Errorf(format, args...)
}

func addToBuffer(level logging.Level, newLog string) {
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
	cleanLevel := strings.ToUpper(strings.TrimSpace(level))
	logLevel, err := logging.LogLevel(cleanLevel)
	if err != nil || logLevel == 0 {
		logLevel = logging.DEBUG
	}

	logMu.RLock()
	defer logMu.RUnlock()

	for i := len(logBuffer) - 1; i >= 0 && len(output) < c; i-- {
		if logBuffer[i].level <= logLevel {
			output = append(output, fmt.Sprintf("%s %s - %s", logBuffer[i].time, logBuffer[i].level, logBuffer[i].log))
		}
	}
	return output
}
