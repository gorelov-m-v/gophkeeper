package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/user/gophkeeper/internal/client/common"
	clientconfig "github.com/user/gophkeeper/internal/client/config"
	"github.com/user/gophkeeper/internal/client/crypto"
	"github.com/user/gophkeeper/internal/client/grpcclient"
	"github.com/user/gophkeeper/internal/client/model"
	"github.com/user/gophkeeper/internal/client/session"
	"github.com/user/gophkeeper/internal/client/store"
	"github.com/user/gophkeeper/pkg/gen"
)

func newUpdateCmd(client grpcclient.GophKeeperClient, cache store.SecretStore, sess *session.Session, enc *crypto.Encryptor, _ *clientconfig.Config) *cobra.Command {
	var (
		secretType string
		meta       string
		username   string
		password   string
		url        string
		text       string
		file       string
		cardNumber string
		holder     string
		expMonth   int
		expYear    int
		cvv        string
	)

	cmd := &cobra.Command{
		Use:   "update [name]",
		Short: "Update an existing secret",
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

			metaValue := meta
			if metaValue == "" {
				metaValue = envelope.Meta
			}

			var (
				body any
				kind gen.DataKind
			)

			switch secretType {
			case "login":
				kind = gen.DataKind_DATA_KIND_LOGIN
				body = model.LoginData{Username: username, Password: password, URL: url}
			case "text":
				kind = gen.DataKind_DATA_KIND_TEXT
				body = model.TextData{Content: text}
			case "binary":
				kind = gen.DataKind_DATA_KIND_BINARY
				content, readErr := os.ReadFile(file)
				if readErr != nil {
					return fmt.Errorf("failed to read file: %w", readErr)
				}
				body = model.BinaryData{Filename: file, Content: content}
			case "card":
				kind = gen.DataKind_DATA_KIND_CARD
				body = model.CardData{
					Number:   cardNumber,
					Holder:   holder,
					CVV:      cvv,
					ExpMonth: expMonth,
					ExpYear:  expYear,
				}
			default:
				return fmt.Errorf("unknown type: %s", secretType)
			}

			encrypted, err := common.EncodeEnvelope(metaValue, body, sess, enc, cliLoginHint)
			if err != nil {
				return err
			}

			secret, err := client.UpdateSecret(cmd.Context(), record.ID, args[0], kind, encrypted, record.Version)
			if err != nil {
				return fmt.Errorf("failed to update secret: %w", err)
			}

			if err := cache.Upsert(userID, common.SecretRecordFromProto(secret)); err != nil {
				return fmt.Errorf("failed to update local cache: %w", err)
			}

			fmt.Println("Secret updated:", secret.GetName())
			return nil
		},
	}

	cmd.Flags().StringVar(&secretType, "type", "", "secret type (login, text, binary, card)")
	cmd.Flags().StringVar(&meta, "meta", "", "optional metadata")
	cmd.Flags().StringVar(&username, "username", "", "login username")
	cmd.Flags().StringVar(&password, "password", "", "login password")
	cmd.Flags().StringVar(&url, "url", "", "login URL")
	cmd.Flags().StringVar(&text, "text", "", "text content")
	cmd.Flags().StringVar(&file, "file", "", "path to binary file")
	cmd.Flags().StringVar(&cardNumber, "card-number", "", "card number")
	cmd.Flags().StringVar(&holder, "holder", "", "card holder name")
	cmd.Flags().IntVar(&expMonth, "exp-month", 0, "card expiration month")
	cmd.Flags().IntVar(&expYear, "exp-year", 0, "card expiration year")
	cmd.Flags().StringVar(&cvv, "cvv", "", "card CVV")
	_ = cmd.MarkFlagRequired("type")

	return cmd
}
