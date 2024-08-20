package ssh

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/2017fighting/devssh/pkg/client"
	"github.com/2017fighting/devssh/pkg/kubernetes"

	"github.com/loft-sh/log"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"

	"github.com/2017fighting/devssh/pkg/agent"
	"github.com/loft-sh/devpod/cmd/machine"
	client2 "github.com/loft-sh/devpod/pkg/client"
	devssh "github.com/loft-sh/devpod/pkg/ssh"

	// "github.com/loft-sh/devpod/pkg/tunnel"
	"github.com/spf13/cobra"
)

type SSHCmd struct {
	NameSpace string
	Service   string

	// Command string
	User string
	// WorkDir string
}

// devssh ssh --
func NewSSHCmd() *cobra.Command {
	cmd := &SSHCmd{}
	sshCmd := &cobra.Command{
		Use:   "ssh",
		Short: "Starts a new ssh session to a container",
		RunE: func(_ *cobra.Command, args []string) error {
			ctx := context.Background()
			return cmd.Run(ctx, log.Default.ErrorStreamOnly())
		},
	}
	sshCmd.Flags().StringVar(&cmd.NameSpace, "ns", "", "The k8s namespace of the container")
	sshCmd.Flags().StringVar(&cmd.Service, "svc", "", "The k8s service of the container")
	// sshCmd.Flags().StringVar(&cmd.Command, "command", "", "The command to execute within the workspace")
	sshCmd.Flags().StringVar(&cmd.User, "user", "root", "The user of the pod to use")
	// sshCmd.Flags().StringVar(&cmd.WorkDir, "workdir", "", "The working directory in the container")
	return sshCmd
}

func (cmd *SSHCmd) Run(ctx context.Context, log log.Logger) error {
	// add ssh keys to agent
	err := devssh.AddPrivateKeysToAgent(ctx, log)
	if err != nil {
		log.Debugf("Error adding private keys to ssh-agent: %v", err)
	}
	client := client.NewWorkspaceClient(cmd.NameSpace, cmd.Service, log)

	// default to root
	if cmd.User == "" {
		cmd.User = "root"
	}

	if cmd.NameSpace == "" {
		return fmt.Errorf("please specify k8s namespace")
	}
	if cmd.Service == "" {
		return fmt.Errorf("please specify k8s service")
	}

	return cmd.jumpContainer(ctx, client, log)
}

func ensureRunning(
	ctx context.Context,
	client *client.WorkspaceClient,
) error {
	instanceStatus, err := client.Status(ctx)
	if err != nil {
		return err
	}
	if instanceStatus != client2.StatusRunning {
		return fmt.Errorf("svc not running")
	}
	return nil
}

func (cmd *SSHCmd) jumpContainer(ctx context.Context, client *client.WorkspaceClient, log log.Logger) error {
	log.Info("jump container")
	// lock workspace
	unlockOnce := sync.Once{}
	err := client.Lock(ctx)
	if err != nil {
		return err
	}
	defer unlockOnce.Do(client.Unlock)

	// start workspace
	err = ensureRunning(ctx, client)
	if err != nil {
		return err
	}
	kubernetes.Exec(ctx, cmd.NameSpace, cmd.Service)

	return nil

	// tunnel to container
	// TODO:好像没必要起两个ssh-server
	// return tunnel.NewContainerTunnel(client, log).Run(ctx, func(ctx context.Context, containerClient *ssh.Client) error {
	// 	// we have a connection to the container, make sure others can connect as well
	// 	unlockOnce.Do(client.Unlock)
	//
	// 	//start ssh tunnel
	// 	return cmd.startTunnel(ctx, containerClient, client.Namespace, client.Service, log)
	// })

}

func (cmd *SSHCmd) startTunnel(ctx context.Context, containerClient *ssh.Client, namespace string, service string, log log.Logger) error {
	// TODO:start tunnel service(port-forward/docker credetials/git credentials/ssh signature helper/etc.)
	writer := log.ErrorStreamOnly().Writer(logrus.InfoLevel, false)
	defer writer.Close()
	// TODO:GPG agent

	log.Debug("Run outer container tunnel")
	command := fmt.Sprintf("'%s' helper ssh-server --debug", agent.ContainerDevPodHelperLocation)
	if cmd.User != "" && cmd.User != "root" {
		command = fmt.Sprintf("su -c \"%s\" '%s'", command, cmd.User)
	}
	return machine.StartSSHSession(
		ctx,
		cmd.User,
		"",
		true,
		func(ctx context.Context, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
			return devssh.Run(ctx, containerClient, command, stdin, stdout, stderr)
		},
		writer,
	)
}
