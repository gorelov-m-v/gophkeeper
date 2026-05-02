package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	clientconfig "github.com/user/gophkeeper/internal/client/config"
	"github.com/user/gophkeeper/internal/client/crypto"
	"github.com/user/gophkeeper/internal/client/grpcclient"
	"github.com/user/gophkeeper/internal/client/session"
)

func newRegisterCmd(client grpcclient.GophKeeperClient, sess *session.Session, enc *crypto.Encryptor, cfg *clientconfig.Config) *cobra.Command {
	var email string

	cmd := &cobra.Command{
		Use:   "register",
		Short: "Register a new account",
		RunE: func(cmd *cobra.Command, args []string) error {
			password, err := readPassword(cmd, "Account password: ")
			if err != nil {
				return fmt.Errorf("failed to read password: %w", err)
			}

			if err := client.Register(cmd.Context(), email, password); err != nil {
				return fmt.Errorf("registration failed: %w", err)
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

			fmt.Println("Registration successful.")
			return nil
		},
	}

	cmd.Flags().StringVar(&email, "email", "", "account email address")
	_ = cmd.MarkFlagRequired("email")

	return cmd
}

// readPassword reads a password interactively with echo disabled when stdin is
// a terminal, or falls back to reading a line for piped input.
func readPassword(cmd *cobra.Command, prompt string) (string, error) {
	in := cmd.InOrStdin()

	if f, ok := in.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		fmt.Fprint(cmd.OutOrStdout(), prompt)
		pw, err := term.ReadPassword(int(f.Fd()))
		fmt.Fprintln(cmd.OutOrStdout())
		if err != nil {
			return "", err
		}
		return string(pw), nil
	}

	if in == os.Stdin && term.IsTerminal(int(syscall.Stdin)) {
		fmt.Fprint(cmd.OutOrStdout(), prompt)
		pw, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Fprintln(cmd.OutOrStdout())
		if err != nil {
			return "", err
		}
		return string(pw), nil
	}

	var line []byte
	buf := make([]byte, 1)
	for {
		n, err := in.Read(buf)
		if n > 0 {
			if buf[0] == '\n' {
				break
			}
			if buf[0] != '\r' {
				line = append(line, buf[0])
			}
		}
		if err != nil {
			if len(line) > 0 {
				break
			}
			return "", fmt.Errorf("failed to read input")
		}
	}
	return string(line), nil
}
