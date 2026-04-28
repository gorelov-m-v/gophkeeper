// Package database provides PostgreSQL connection and migration utilities.
package database

import (
	"context"
	"errors"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// NewPostgresPool creates a new pgx connection pool for the given DSN.
func NewPostgresPool(ctx context.Context, dsn string, logger *zap.Logger) (*pgxpool.Pool, error) {
	logger.Info("connecting to postgres")
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	logger.Info("postgres connection established")
	return pool, nil
}

// RunMigrations applies all pending database migrations from the given path.
func RunMigrations(dsn, migrationsPath string, logger *zap.Logger) error {
	logger.Info("running database migrations", zap.String("path", migrationsPath))
	m, err := migrate.New("file://"+migrationsPath, dsn)
	if err != nil {
		return err
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}

	logger.Info("database migrations complete")
	return nil
}
