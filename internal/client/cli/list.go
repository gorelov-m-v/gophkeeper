package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	clientconfig "github.com/user/gophkeeper/internal/client/config"
	"github.com/user/gophkeeper/internal/client/crypto"
	"github.com/user/gophkeeper/internal/client/session"
	"github.com/user/gophkeeper/internal/client/store"
	"github.com/user/gophkeeper/pkg/gen"
)

func newListCmd(cache store.SecretStore, sess *session.Session, _ *crypto.Encryptor, _ *clientconfig.Config) *cobra.Command {
	var filterType string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List locally cached secrets",
		RunE: func(cmd *cobra.Command, args []string) error {
			userID, err := currentUserID(sess)
			if err != nil {
				return err
			}

			secrets, err := cache.List(userID)
			if err != nil {
				return err
			}

			var kindFilter gen.DataKind
			if filterType != "" {
				switch filterType {
				case "login":
					kindFilter = gen.DataKind_DATA_KIND_LOGIN
				case "text":
					kindFilter = gen.DataKind_DATA_KIND_TEXT
				case "binary":
					kindFilter = gen.DataKind_DATA_KIND_BINARY
				case "card":
					kindFilter = gen.DataKind_DATA_KIND_CARD
				default:
					return fmt.Errorf("unknown type: %s", filterType)
				}
			}

			sortSecretsByName(secrets)

			fmt.Printf("%-30s %-15s %-25s\n", "NAME", "TYPE", "UPDATED")
			for _, secret := range secrets {
				kind := gen.DataKind(secret.Kind)
				if filterType != "" && kind != kindFilter {
					continue
				}

				fmt.Printf("%-30s %-15s %-25s\n",
					secret.Name,
					kind.String(),
					secret.UpdatedAt.Format("2006-01-02 15:04:05"),
				)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&filterType, "type", "", "filter by type (login, text, binary, card)")

	return cmd
}
