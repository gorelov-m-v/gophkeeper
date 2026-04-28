// Package app wires together all server components and manages the application lifecycle.
package app

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	serverconfig "github.com/user/gophkeeper/internal/server/config"
	"github.com/user/gophkeeper/internal/server/database"
	"github.com/user/gophkeeper/internal/server/grpchandler"
	"github.com/user/gophkeeper/internal/server/interceptor"
	"github.com/user/gophkeeper/internal/server/jwt"
	"github.com/user/gophkeeper/internal/server/repository"
	"github.com/user/gophkeeper/internal/server/service"
	"github.com/user/gophkeeper/pkg/gen"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// App holds the configured gRPC server and its dependencies.
type App struct {
	grpcServer *grpc.Server
	cfg        *serverconfig.Config
	pool       *pgxpool.Pool
	logger     *zap.Logger
}

// New creates a fully wired App from the given configuration.
func New(cfg *serverconfig.Config, logger *zap.Logger) (*App, error) {
	ctx := context.Background()

	pool, err := database.NewPostgresPool(ctx, cfg.DatabaseDSN, logger)
	if err != nil {
		return nil, fmt.Errorf("database connection: %w", err)
	}

	if err := database.RunMigrations(cfg.DatabaseDSN, cfg.MigrationsPath, logger); err != nil {
		pool.Close()
		return nil, fmt.Errorf("migrations: %w", err)
	}

	tokenManager := jwt.NewTokenManager(cfg.JWTSecret, cfg.JWTAccessTTL, cfg.JWTRefreshTTL)
	userRepo := repository.NewUserRepo(pool)
	secretRepo := repository.NewSecretRepo(pool)

	authService := service.NewAuthService(userRepo, tokenManager)
	secretsService := service.NewSecretsService(secretRepo)

	authHandler := grpchandler.NewAuthHandler(authService)
	secretsHandler := grpchandler.NewSecretsHandler(secretsService)

	serverOptions := []grpc.ServerOption{
		grpc.UnaryInterceptor(interceptor.AuthUnaryInterceptor(tokenManager, logger)),
	}

	if !cfg.AllowInsecureGRPC {
		certificate, err := tls.LoadX509KeyPair(cfg.TLSCertFile, cfg.TLSKeyFile)
		if err != nil {
			pool.Close()
			return nil, fmt.Errorf("load TLS key pair: %w", err)
		}

		serverOptions = append(serverOptions, grpc.Creds(credentials.NewTLS(&tls.Config{
			MinVersion:   tls.VersionTLS12,
			Certificates: []tls.Certificate{certificate},
		})))
	}

	grpcServer := grpc.NewServer(serverOptions...)
	gen.RegisterAuthServiceServer(grpcServer, authHandler)
	gen.RegisterSecretsServiceServer(grpcServer, secretsHandler)

	return &App{
		grpcServer: grpcServer,
		cfg:        cfg,
		pool:       pool,
		logger:     logger,
	}, nil
}

// Run starts the gRPC server and blocks until a shutdown signal is received.
func (a *App) Run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	listener, err := net.Listen("tcp", a.cfg.GRPCAddress)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	go func() {
		<-ctx.Done()
		a.logger.Info("shutting down gRPC server")
		a.grpcServer.GracefulStop()
		a.pool.Close()
		a.logger.Info("database pool closed")
	}()

	a.logger.Info("gRPC server listening", zap.String("address", a.cfg.GRPCAddress), zap.Bool("insecure", a.cfg.AllowInsecureGRPC))
	return a.grpcServer.Serve(listener)
}
