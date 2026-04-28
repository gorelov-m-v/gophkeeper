// Package main is the entry point for the GophKeeper server.
package main

import (
	"os"

	"github.com/user/gophkeeper/internal/server/app"
	serverconfig "github.com/user/gophkeeper/internal/server/config"
	"go.uber.org/zap"
)

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	cfg, err := serverconfig.Load(os.Args[1:])
	if err != nil {
		logger.Fatal("invalid configuration", zap.Error(err))
	}

	application, err := app.New(cfg, logger)
	if err != nil {
		logger.Fatal("failed to initialize application", zap.Error(err))
	}

	if err := application.Run(); err != nil {
		logger.Fatal("server error", zap.Error(err))
	}
}
