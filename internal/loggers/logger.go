package loggers

import (
	"fmt"
	"path/filepath"
	"runtime"

	"github.com/lechgu/tichy/internal/config"
	"github.com/samber/do/v2"
	"github.com/sirupsen/logrus"
)

func New(di do.Injector) (*logrus.Logger, error) {
	cfg, err := do.Invoke[*config.Config](di)
	if err != nil {
		return nil, err
	}

	logger := logrus.New()

	level, err := logrus.ParseLevel(cfg.LogLevel)
	if err != nil {
		level = logrus.InfoLevel
	}
	logger.SetLevel(level)

	logger.SetReportCaller(true)
	logger.SetFormatter(&logrus.TextFormatter{
		CallerPrettyfier: func(frame *runtime.Frame) (function string, file string) {
			filename := filepath.Base(frame.File)
			return "", fmt.Sprintf("%s:%d", filename, frame.Line)
		},
	})

	return logger, nil
}
