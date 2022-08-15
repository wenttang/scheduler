package main

import (
	"net/http"
	"os"

	daprd "github.com/dapr/go-sdk/service/http"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/wenttang/scheduler/internal/config"
	"github.com/wenttang/scheduler/internal/scheduler"
	"github.com/wenttang/scheduler/pkg/actor"
)

func main() {
	Main()
}

const (
	logFormatJson = "json"
	logLevelDebug = "debug"
	logLevelInfo  = "info"
	logLevelWarn  = "warn"
	logLevelError = "error"
)

func Main() {
	conf, err := config.Get()
	if err != nil {
		panic(err)
	}

	var logger log.Logger

	logger = log.NewJSONLogger(log.NewSyncWriter(os.Stdout))

	switch conf.LogLevel {
	case logLevelDebug:
		logger = level.NewFilter(logger, level.AllowDebug())
	case logLevelInfo:
		logger = level.NewFilter(logger, level.AllowInfo())
	case logLevelWarn:
		logger = level.NewFilter(logger, level.AllowWarn())
	case logLevelError:
		logger = level.NewFilter(logger, level.AllowError())
	default:
		logger = level.NewFilter(logger, level.AllowDebug())
	}

	logger = log.With(logger, "ts", log.DefaultTimestamp)
	logger = log.With(logger, "caller", log.DefaultCaller)

	daprClient, err := actor.NewDapr()
	if err != nil {
		level.Error(logger).Log("message", err.Error())
		os.Exit(1)
	}

	s := daprd.NewService(conf.Addr)
	s.RegisterActorImplFactory(scheduler.New(conf, logger, daprClient))

	level.Info(logger).Log("message", "Starting...")
	if err := s.Start(); err != nil && err != http.ErrServerClosed {
		level.Error(logger).Log("message", err.Error())
	}
}
