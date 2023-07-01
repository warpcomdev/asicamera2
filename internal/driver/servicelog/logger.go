package servicelog

import (
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/kardianos/service"
	"go.uber.org/zap"
	"gopkg.in/natefinch/lumberjack.v2"
)

type lumberjackSink struct {
	*lumberjack.Logger
}

func (lumberjackSink) Sync() error {
	return nil
}

type Attrib func(sb *strings.Builder)

type logger struct {
	logger service.Logger
	debug  bool
	attrs  []Attrib
}

func printer(name string, val interface{}) Attrib {
	return func(sb *strings.Builder) {
		sb.WriteString(", ")
		sb.WriteString(name)
		sb.WriteString("=")
		fmt.Fprintf(sb, "%v", val)
	}
}

func String(name, value string) Attrib {
	return printer(name, value)
}

func Error(err error) Attrib {
	return printer("error", err)
}

func Bool(name string, value bool) Attrib {
	return printer(name, value)
}

func Any(name string, value interface{}) Attrib {
	return printer(name, value)
}

func Int(name string, value int) Attrib {
	return printer(name, value)
}

func Time(name string, value time.Time) Attrib {
	return printer(name, value)
}

func Duration(name string, value time.Duration) Attrib {
	return printer(name, value)
}

func New(root service.Logger, debug bool) Logger {
	zap.RegisterSink("lumberjack", func(u *url.URL) (zap.Sink, error) {
		return lumberjackSink{
			Logger: &lumberjack.Logger{
				Filename: u.Path,
			},
		}, nil
	})

	var config zap.Config
	if debug {
		config = zap.NewDevelopmentConfig()
	} else {
		config = zap.NewProductionConfig()
	}
	config.OutputPaths = []string{"lumberjack://asicamera2.log"}
	logger, err := config.Build()
	if err != nil {
		panic(err)
	}
	return logger
}

type Logger interface {
	With(attrs ...Attrib) Logger
	Info(msg string, attrs ...Attrib)
	Error(msg string, attrs ...Attrib)
	Warn(msg string, attrs ...Attrib)
	Debug(msg string, attrs ...Attrib)
	Fatal(msg string, attrs ...Attrib)
}

func (l *logger) String(msg string, attrs ...Attrib) string {
	var sb strings.Builder
	sb.WriteString(msg)
	if l != nil && l.attrs != nil {
		for _, a := range l.attrs {
			a(&sb)
		}
	}
	if attrs != nil && len(attrs) > 0 {
		for _, a := range attrs {
			a(&sb)
		}
	}
	return sb.String()
}

func (l *logger) Info(msg string, attrs ...Attrib) {
	message := l.String(msg, attrs...)
	if l != nil {
		l.logger.Info(message)
	} else {
		log.Println(message)
	}
}

func (l *logger) Error(msg string, attrs ...Attrib) {
	message := l.String(msg, attrs...)
	if l != nil {
		l.logger.Error(message)
	} else {
		log.Println(message)
	}
}

func (l *logger) Fatal(msg string, attrs ...Attrib) {
	message := l.String(msg, attrs...)
	if l != nil {
		l.logger.Error(message)
		panic(msg)
	} else {
		log.Fatal(message)
	}
}

func (l *logger) Warn(msg string, attrs ...Attrib) {
	message := l.String(msg, attrs...)
	if l != nil {
		l.logger.Warning(message)
	} else {
		log.Println(message)
	}
}

func (l *logger) Debug(msg string, attrs ...Attrib) {
	if l.debug {
		message := l.String(msg, attrs...)
		if l != nil {
			l.logger.Info(message)
		} else {
			log.Println(message)
		}
	}
}

func (l *logger) With(attrs ...Attrib) Logger {
	newLogger := &logger{
		logger: nil,
		debug:  false,
		attrs:  nil,
	}
	if l != nil {
		newLogger.logger = l.logger
		newLogger.debug = l.debug
	}
	if l != nil && l.attrs != nil && len(l.attrs) > 0 {
		newLogger.attrs = make([]Attrib, 0, len(l.attrs)+len(attrs))
		newLogger.attrs = append(newLogger.attrs, l.attrs...)
	}
	if attrs != nil && len(attrs) > 0 {
		newLogger.attrs = append(newLogger.attrs, attrs...)
	}
	return newLogger
}
