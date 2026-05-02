package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/user/gophkeeper/internal/client/common"
	clientconfig "github.com/user/gophkeeper/internal/client/config"
	"github.com/user/gophkeeper/internal/client/crypto"
	"github.com/user/gophkeeper/internal/client/model"
	"github.com/user/gophkeeper/internal/client/session"
	"github.com/user/gophkeeper/internal/client/store"
	"github.com/user/gophkeeper/pkg/gen"
)

func newGetCmd(cache store.SecretStore, sess *session.Session, enc *crypto.Encryptor, _ *clientconfig.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get [name]",
		Short: "Get a secret by name from the local cache",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			userID, err := common.CurrentUserID(sess, cliLoginHint)
			if err != nil {
				return err
			}

			record, err := cache.GetByName(userID, args[0])
			if err != nil {
				return err
			}

			envelope, err := common.DecodeEnvelope(record.Payload, sess, enc, cliLoginHint)
			if err != nil {
				return err
			}

			fmt.Println("Name:", record.Name)
			fmt.Println("Type:", gen.DataKind(record.Kind).String())

			switch gen.DataKind(record.Kind) {
			case gen.DataKind_DATA_KIND_LOGIN:
				body, err := common.DecodeBody[model.LoginData](envelope)
				if err != nil {
					return fmt.Errorf("failed to parse login data: %w", err)
				}
				fmt.Println("Username:", body.Username)
				fmt.Println("Password:", body.Password)
				fmt.Println("URL:", body.URL)
			case gen.DataKind_DATA_KIND_TEXT:
				body, err := common.DecodeBody[model.TextData](envelope)
				if err != nil {
					return fmt.Errorf("failed to parse text data: %w", err)
				}
				fmt.Println("Content:", body.Content)
			case gen.DataKind_DATA_KIND_BINARY:
				body, err := common.DecodeBody[model.BinaryData](envelope)
				if err != nil {
					return fmt.Errorf("failed to parse binary data: %w", err)
				}
				fmt.Println("Filename:", body.Filename)
				fmt.Printf("Size: %d bytes\n", len(body.Content))
			case gen.DataKind_DATA_KIND_CARD:
				body, err := common.DecodeBody[model.CardData](envelope)
				if err != nil {
					return fmt.Errorf("failed to parse card data: %w", err)
				}
				fmt.Println("Number:", body.Number)
				fmt.Println("Holder:", body.Holder)
				fmt.Printf("Expires: %02d/%d\n", body.ExpMonth, body.ExpYear)
				fmt.Println("CVV:", body.CVV)
			}

			if envelope.Meta != "" {
				fmt.Println("Metadata:", envelope.Meta)
			}

			return nil
		},
	}

	return cmd
}
