package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

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
			userID, err := currentUserID(sess)
			if err != nil {
				return err
			}

			record, err := cache.GetByName(userID, args[0])
			if err != nil {
				return err
			}

			envelope, err := decodeEnvelope(record.Payload, sess, enc)
			if err != nil {
				return err
			}

			fmt.Println("Name:", record.Name)
			fmt.Println("Type:", gen.DataKind(record.Kind).String())

			switch gen.DataKind(record.Kind) {
			case gen.DataKind_DATA_KIND_LOGIN:
				var body model.LoginData
				if err := json.Unmarshal(envelope.Body, &body); err != nil {
					return fmt.Errorf("failed to parse login data: %w", err)
				}
				fmt.Println("Username:", body.Username)
				fmt.Println("Password:", body.Password)
				fmt.Println("URL:", body.URL)
			case gen.DataKind_DATA_KIND_TEXT:
				var body model.TextData
				if err := json.Unmarshal(envelope.Body, &body); err != nil {
					return fmt.Errorf("failed to parse text data: %w", err)
				}
				fmt.Println("Content:", body.Content)
			case gen.DataKind_DATA_KIND_BINARY:
				var body model.BinaryData
				if err := json.Unmarshal(envelope.Body, &body); err != nil {
					return fmt.Errorf("failed to parse binary data: %w", err)
				}
				fmt.Println("Filename:", body.Filename)
				fmt.Printf("Size: %d bytes\n", len(body.Content))
			case gen.DataKind_DATA_KIND_CARD:
				var body model.CardData
				if err := json.Unmarshal(envelope.Body, &body); err != nil {
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
