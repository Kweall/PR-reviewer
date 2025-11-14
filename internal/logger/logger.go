package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"
)

type stdLogger struct {
	out      *log.Logger
	workerID string
	level    logLevel
}

type logLevel int

const (
	levelSuccess logLevel = iota
	levelInfo
	levelWarn
	levelError
)

func NewStdLogger(w io.Writer, levelStr string) Logger {
	l := &stdLogger{
		out:   log.New(w, "", 0),
		level: parseLevel(levelStr),
	}
	return l
}

func NewDefaultLogger() Logger {
	return NewStdLogger(os.Stdout, "info")
}

func parseLevel(s string) logLevel {
	switch strings.ToLower(s) {
	case "success":
		return levelSuccess
	case "warn":
		return levelWarn
	case "error":
		return levelError
	default:
		return levelInfo
	}
}

func (l *stdLogger) WithWorker(workerID string) Logger {
	return &stdLogger{
		out:      l.out,
		workerID: workerID,
		level:    l.level,
	}
}

func (l *stdLogger) Success(msg string, kv ...any) {
	if l.level <= levelInfo {
		l.print("\033[32mSUCCESS\033[0m", msg, kv...)
	}
}
func (l *stdLogger) Info(msg string, kv ...any) {
	if l.level <= levelInfo {
		l.print("INFO", msg, kv...)
	}
}
func (l *stdLogger) Warn(msg string, kv ...any) {
	if l.level <= levelWarn {
		l.print("\033[33mWARN\033[0m", msg, kv...)
	}
}
func (l *stdLogger) Error(msg string, kv ...any) {
	if l.level <= levelError {
		l.print("\033[31mERROR\033[0m", msg, kv...)
	}
}

func (l *stdLogger) print(levelStr, msg string, kv ...any) {
	ts := time.Now().Format("2006/01/02 15:04:05")

	workerTag := ""
	if l.workerID != "" {
		workerTag = fmt.Sprintf("[%s] ", l.workerID)
	}

	kvStr := ""
	if len(kv)%2 == 0 && len(kv) > 0 {
		parts := make([]string, 0, len(kv)/2)
		for i := 0; i < len(kv); i += 2 {
			k := fmt.Sprint(kv[i])
			v := fmt.Sprint(kv[i+1])
			parts = append(parts, fmt.Sprintf("%s=%s", k, v))
		}
		kvStr = " " + strings.Join(parts, " ")
	} else if len(kv) > 0 {
		parts := make([]string, 0, len(kv))
		for _, x := range kv {
			parts = append(parts, fmt.Sprint(x))
		}
		kvStr = " " + strings.Join(parts, " ")
	}

	line := fmt.Sprintf("%s %s%s%s%s", ts, levelStr, " ", workerTag, msg+kvStr)

	l.out.Print(line)
}
