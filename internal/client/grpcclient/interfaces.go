// Package grpcclient wraps gRPC service clients for the GophKeeper server.
package grpcclient

import (
	"context"
	"time"

	"github.com/user/gophkeeper/pkg/gen"
)

// GophKeeperClient defines the methods that CLI and TUI commands use to
// interact with the GophKeeper server.
type GophKeeperClient interface {
	Register(ctx context.Context, email, password string) error
	Login(ctx context.Context, email, password string) error
	RefreshToken(ctx context.Context) error
	CreateSecret(ctx context.Context, name string, kind gen.DataKind, payload []byte) (*gen.SecretItem, error)
	GetSecret(ctx context.Context, id string) (*gen.SecretItem, error)
	ListSecrets(ctx context.Context) ([]*gen.SecretItem, error)
	UpdateSecret(ctx context.Context, id, name string, kind gen.DataKind, payload []byte, version int64) (*gen.SecretItem, error)
	DeleteSecret(ctx context.Context, id string, version int64) error
	SyncSecrets(ctx context.Context, since time.Time) ([]*gen.SecretItem, time.Time, error)
	Close() error
}
