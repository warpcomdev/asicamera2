package servicelog

import (
	"net/url"
	"path/filepath"
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

type Attrib = zap.Field
type Logger struct {
	*zap.Logger
}

func String(name, value string) Attrib {
	return zap.String(name, value)
}

func Error(err error) Attrib {
	return zap.Error(err)
}

func Bool(name string, value bool) Attrib {
	return zap.Bool(name, value)
}

func Any(name string, value interface{}) Attrib {
	return zap.Any(name, value)
}

func Int(name string, value int) Attrib {
	return zap.Int(name, value)
}

func Time(name string, value time.Time) Attrib {
	return zap.Time(name, value)
}

func Duration(name string, value time.Duration) Attrib {
	return zap.Duration(name, value)
}

func New(root service.Logger, logDir string, debug bool) (Logger, error) {
	zap.RegisterSink("lumberjack", func(u *url.URL) (zap.Sink, error) {
		return lumberjackSink{
			Logger: &lumberjack.Logger{
				Filename:   filepath.Join(logDir, filepath.Base(u.Path)),
				MaxSize:    8 * 1024 * 1024,
				MaxBackups: 128,
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
		return Logger{}, err
	}

	// Avoid stack traces below panic level
	logger = logger.WithOptions(zap.AddStacktrace(zap.DPanicLevel))
	return Logger{Logger: logger}, nil
}

func (l Logger) With(fields ...Attrib) Logger {
	return Logger{
		Logger: l.Logger.With(fields...),
	}
}
