package sshserver

import (
	"context"
	"os"

	"github.com/loft-sh/log"
	"github.com/loft-sh/ssh"

	helperssh "github.com/loft-sh/devpod/pkg/ssh/server"
	"github.com/loft-sh/devpod/pkg/stdio"
	"github.com/spf13/cobra"
)

type SSHServerCmd struct {
	WorkDir string
}

func NewSSHServerCmd() *cobra.Command {
	cmd := &SSHServerCmd{}
	sshServerCmd := &cobra.Command{
		Use:   "ssh-server",
		Short: "Starts a new ssh server",
		RunE: func(_ *cobra.Command, args []string) error {
			ctx := context.Background()
			return cmd.Run(ctx)
		},
	}
	sshServerCmd.Flags().StringVar(&cmd.WorkDir, "workdir", "/workspaces", "The working directory in the container")
	return sshServerCmd

}

func (c *SSHServerCmd) Run(ctx context.Context) error {
	var (
		keys    []ssh.PublicKey
		hostKey []byte
	)
	server, err := helperssh.NewServer("0.0.0.0:8022", hostKey, keys, c.WorkDir, log.Default.ErrorStreamOnly())
	if err != nil {
		return err
	}
	lis := stdio.NewStdioListener(os.Stdin, os.Stdout, true)
	return server.Serve(lis)
}
