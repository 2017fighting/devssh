package cmd

import (
	"os"
	"os/exec"

	"github.com/2017fighting/devssh/cmd/agent"
	ssh2 "github.com/2017fighting/devssh/cmd/ssh"
	sshserver "github.com/2017fighting/devssh/cmd/ssh-server"
	log2 "github.com/loft-sh/log"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
)

func buildRoot() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "devssh",
		Short:         "DevSSH",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.AddCommand(ssh2.NewSSHCmd())
	cmd.AddCommand(sshserver.NewSSHServerCmd())
	cmd.AddCommand(agent.NewAgentCmd())
	return cmd
}

func Execute() {
	rootCmd := buildRoot()
	err := rootCmd.Execute()
	if err != nil {
		//nolint:all
		if sshExitErr, ok := err.(*ssh.ExitError); ok {
			os.Exit(sshExitErr.ExitStatus())
		}
		//nolint:all
		if execExitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(execExitErr.ExitCode())
		}
		log2.Default.Fatalf("%+v", err)
	}

}
