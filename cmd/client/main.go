// Package main is the entry point for the GophKeeper CLI client.
package main

import (
	"os"

	"github.com/user/gophkeeper/internal/client/cli"
	clientconfig "github.com/user/gophkeeper/internal/client/config"
	"github.com/user/gophkeeper/internal/client/crypto"
	"github.com/user/gophkeeper/internal/client/grpcclient"
	"github.com/user/gophkeeper/internal/client/session"
	"github.com/user/gophkeeper/internal/client/store"
	"go.uber.org/zap"
)

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	cfg, remainingArgs, err := clientconfig.Load(os.Args[1:])
	if err != nil {
		logger.Fatal("failed to load config", zap.Error(err))
	}

	sess := session.NewSession()

	client, err := grpcclient.NewClient(cfg.ServerAddress, sess, cfg.Insecure, cfg.TLSCAFile, logger)
	if err != nil {
		logger.Fatal("failed to create client", zap.Error(err))
	}
	defer client.Close()

	enc := crypto.NewEncryptor()
	cache := store.NewFileStore(cfg.ConfigDir)

	rootCmd := cli.NewRootCmd(client, cache, sess, enc, cfg)
	rootCmd.SetArgs(remainingArgs)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
