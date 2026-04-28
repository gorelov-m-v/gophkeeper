package cli

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	clientconfig "github.com/user/gophkeeper/internal/client/config"
	"github.com/user/gophkeeper/internal/client/crypto"
	"github.com/user/gophkeeper/internal/client/grpcclient"
	"github.com/user/gophkeeper/internal/client/session"
	"github.com/user/gophkeeper/internal/client/store"
)

func newDeleteCmd(client grpcclient.GophKeeperClient, cache store.SecretStore, sess *session.Session, _ *crypto.Encryptor, _ *clientconfig.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete [name]",
		Short: "Delete a secret by name",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			userID, err := currentUserID(sess)
			if err != nil {
				return err
			}

			record, err := cache.GetByName(userID, args[0])
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Are you sure you want to delete '%s'? [y/N]: ", args[0])
			scanner := bufio.NewScanner(cmd.InOrStdin())
			scanner.Scan()
			answer := strings.TrimSpace(scanner.Text())
			if !strings.HasPrefix(answer, "y") && !strings.HasPrefix(answer, "Y") {
				fmt.Fprintln(cmd.OutOrStdout(), "Aborted.")
				return nil
			}

			if err := client.DeleteSecret(cmd.Context(), record.ID, record.Version); err != nil {
				return fmt.Errorf("failed to delete secret: %w", err)
			}
			if err := cache.Delete(userID, record.ID); err != nil {
				return fmt.Errorf("failed to update local cache: %w", err)
			}

			fmt.Println("Secret deleted:", args[0])
			return nil
		},
	}

	return cmd
}
