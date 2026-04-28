package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	clientconfig "github.com/user/gophkeeper/internal/client/config"
	"github.com/user/gophkeeper/internal/client/crypto"
	"github.com/user/gophkeeper/internal/client/grpcclient"
	"github.com/user/gophkeeper/internal/client/session"
	"github.com/user/gophkeeper/internal/client/store"
)

func newSyncCmd(client grpcclient.GophKeeperClient, cache *store.FileStore, sess *session.Session, _ *crypto.Encryptor, _ *clientconfig.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Synchronize secrets into the local cache",
		RunE: func(cmd *cobra.Command, args []string) error {
			userID, err := currentUserID(sess)
			if err != nil {
				return err
			}

			since, err := cache.LoadCursor(userID)
			if err != nil {
				return fmt.Errorf("failed to read sync cursor: %w", err)
			}

			secrets, serverTime, err := client.SyncSecrets(cmd.Context(), since)
			if err != nil {
				return fmt.Errorf("sync failed: %w", err)
			}

			updated := 0
			deleted := 0
			for _, secret := range secrets {
				if secret.GetDeleted() {
					deleted++
					if err := cache.Delete(userID, secret.GetId()); err != nil && err != store.ErrCacheNotInitialized {
						return fmt.Errorf("failed to delete local cache entry: %w", err)
					}
					continue
				}

				updated++
				if err := cache.Upsert(userID, secretRecordFromProto(secret)); err != nil {
					return fmt.Errorf("failed to update local cache: %w", err)
				}
			}

			if err := cache.SaveCursor(userID, serverTime); err != nil {
				return fmt.Errorf("failed to store sync cursor: %w", err)
			}

			if len(secrets) == 0 {
				fmt.Println("Already up to date.")
				return nil
			}

			fmt.Printf("Synced %d secret(s): %d updated, %d deleted.\n", len(secrets), updated, deleted)
			return nil
		},
	}

	return cmd
}
