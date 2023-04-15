package main

import (
	"log"

	"github.com/Revolyssup/apisix-session-manager/session"
	"github.com/apache/apisix-go-plugin-runner/pkg/plugin"
	"github.com/apache/apisix-go-plugin-runner/pkg/runner"
	"go.uber.org/zap/zapcore"
)

func main() {
	cfg := runner.RunnerConfig{
		LogLevel: zapcore.DebugLevel,
	}
	if err := plugin.RegisterPlugin(session.New(cfg)); err != nil {
		log.Fatalf("failed to register plugin: %s", err.Error())
	}
	runner.Run(cfg)
}
