// Package cli implements the command-line interface for the GophKeeper client.
package cli

import (
	"path/filepath"

	"github.com/spf13/cobra"

	clientconfig "github.com/user/gophkeeper/internal/client/config"
	"github.com/user/gophkeeper/internal/client/crypto"
	"github.com/user/gophkeeper/internal/client/grpcclient"
	"github.com/user/gophkeeper/internal/client/session"
	"github.com/user/gophkeeper/internal/client/store"
	"github.com/user/gophkeeper/internal/version"
)

// NewRootCmd creates the root cobra command with all subcommands attached.
func NewRootCmd(client grpcclient.GophKeeperClient, cache *store.FileStore, sess *session.Session, enc *crypto.Encryptor, cfg *clientconfig.Config) *cobra.Command {
	root := &cobra.Command{
		Use:   "gophkeeper",
		Short: "GophKeeper secret manager client",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			sessionPath := filepath.Join(cfg.ConfigDir, "session.json")
			_ = sess.LoadFromDisk(sessionPath)
			return nil
		},
	}

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print build version information",
		Run: func(cmd *cobra.Command, args []string) {
			version.Print()
		},
	}

	root.AddCommand(versionCmd)
	root.AddCommand(newRegisterCmd(client, sess, enc, cfg))
	root.AddCommand(newLoginCmd(client, sess, enc, cfg))
	root.AddCommand(newAddCmd(client, cache, sess, enc, cfg))
	root.AddCommand(newGetCmd(cache, sess, enc, cfg))
	root.AddCommand(newListCmd(cache, sess, enc, cfg))
	root.AddCommand(newUpdateCmd(client, cache, sess, enc, cfg))
	root.AddCommand(newDeleteCmd(client, cache, sess, enc, cfg))
	root.AddCommand(newSyncCmd(client, cache, sess, enc, cfg))
	root.AddCommand(newTUICmd(client, cache, sess, enc, cfg))

	return root
}
