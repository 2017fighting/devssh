package agent

import "github.com/spf13/cobra"

func NewAgentCmd() *cobra.Command {
	agentCmd := &cobra.Command{
		Use:   "agent",
		Short: "DevSSH Agent",
	}
	agentCmd.AddCommand(NewCSCmd())
	agentCmd.AddCommand(NewGitCredentialsCmd())
	return agentCmd
}
