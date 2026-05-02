package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	clientconfig "github.com/user/gophkeeper/internal/client/config"
	"github.com/user/gophkeeper/internal/client/crypto"
	"github.com/user/gophkeeper/internal/client/grpcclient"
	"github.com/user/gophkeeper/internal/client/session"
)

func newLoginCmd(client grpcclient.GophKeeperClient, sess *session.Session, enc *crypto.Encryptor, cfg *clientconfig.Config) *cobra.Command {
	var email string

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Log in to an existing account",
		RunE: func(cmd *cobra.Command, args []string) error {
			password, err := readPassword(cmd, "Account password: ")
			if err != nil {
				return fmt.Errorf("failed to read password: %w", err)
			}

			if err := client.Login(cmd.Context(), email, password); err != nil {
				return fmt.Errorf("login failed: %w", err)
			}
			sess.SetUserEmail(email)

			masterPassword, err := readPassword(cmd, "Master password: ")
			if err != nil {
				return fmt.Errorf("failed to read master password: %w", err)
			}

			salt := enc.DeriveUserSalt(email)
			key := enc.DeriveKey(masterPassword, salt)
			sess.SetSalt(salt)
			sess.SetEncryptionKey(key)

			sessionPath := filepath.Join(cfg.ConfigDir, "session.json")
			if err := sess.SaveToDisk(sessionPath); err != nil {
				return fmt.Errorf("failed to save session: %w", err)
			}

			fmt.Println("Login successful.")
			return nil
		},
	}

	cmd.Flags().StringVar(&email, "email", "", "account email address")
	_ = cmd.MarkFlagRequired("email")

	return cmd
}
