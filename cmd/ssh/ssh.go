package ssh

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/2017fighting/devssh/pkg/agent"
	"github.com/2017fighting/devssh/pkg/client"
	"github.com/2017fighting/devssh/pkg/kubernetes"
	"github.com/mattn/go-isatty"
	"github.com/pkg/errors"

	"github.com/loft-sh/log"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	"golang.org/x/term"

	// "github.com/loft-sh/devpod/cmd/machine"
	// "github.com/loft-sh/devpod/pkg/agent/tunnelserver"
	"github.com/2017fighting/devssh/pkg/agent/tunnelserver"
	client2 "github.com/loft-sh/devpod/pkg/client"
	"github.com/loft-sh/devpod/pkg/netstat"
	devssh "github.com/loft-sh/devpod/pkg/ssh"

	// "github.com/loft-sh/devpod/pkg/tunnel"
	devsshagent "github.com/loft-sh/devpod/pkg/ssh/agent"
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

	return cmd.jumpContainer(ctx, client)
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

func (cmd *SSHCmd) startExtraService(ctx context.Context, sshClient *ssh.Client) error {
	log.Default.Info("init extra service")
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		return err
	}
	defer stdoutWriter.Close()

	stdinReader, stdinWriter, err := os.Pipe()
	if err != nil {
		return err
	}
	defer stdinWriter.Close()

	// start server on stdio
	cancelCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// create a port forwarder
	var forwarder netstat.Forwarder
	// if true {
	// 	forwarder = newForwarder(sshClient, append(forwardedPorts, fmt.Sprintf("%d", openvscode.DefaultVSCodePort)), log.Default)
	// }
	errChan := make(chan error, 1)
	go func() {
		defer cancel()
		log.Default.Infof("init extra service server")
		// forward credentials to container
		err = tunnelserver.RunServicesServer(
			cancelCtx,
			stdoutReader,
			stdinWriter,
			true,
			true,
			forwarder,
			log.Default,
			tunnelserver.WithGitCredentialsOverride("", ""),
		)
		if err != nil {
			errChan <- errors.Wrap(err, "run tunnel server")
		}
		close(errChan)
	}()

	// run credentials server
	writer := log.Default.ErrorStreamOnly().Writer(logrus.DebugLevel, false)
	defer writer.Close()

	command := fmt.Sprintf("'%s' agent credentials-server --user '%s'", agent.ContainerDevPodHelperLocation, cmd.User)
	log.Default.Infof(command)

	err = devssh.Run(cancelCtx, sshClient, command, stdinReader, stdoutWriter, writer)
	log.Default.Info("run agent credentials-server")
	if err != nil {
		return err
	}
	err = <-errChan
	if err != nil {
		return err
	}
	return nil
}

func (cmd *SSHCmd) startService(ctx context.Context, sshClient *ssh.Client, stderr io.Writer) error {
	// extra service
	go cmd.startExtraService(ctx, sshClient)

	session, err := sshClient.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()
	var (
		stdout io.Writer = os.Stdout
		stdin  io.Reader = os.Stdin
	)

	// request agent forwarding
	authSock := devsshagent.GetSSHAuthSocket()
	if authSock != "" {
		err = devsshagent.ForwardToRemote(sshClient, authSock)
		if err != nil {
			return errors.Errorf("forward agent: %v", err)
		}

		err = devsshagent.RequestAgentForwarding(session)
		if err != nil {
			return errors.Errorf("request agent forwarding: %v", err)
		}
		log.Default.Infof("forward auth sock success")
	}

	stdoutFile, validOut := stdout.(*os.File)
	stdinFile, validIn := stdin.(*os.File)
	if validOut && validIn && isatty.IsTerminal(stdoutFile.Fd()) {
		state, err := term.MakeRaw(int(stdinFile.Fd()))
		if err != nil {
			return err
		}
		defer func() {
			_ = term.Restore(int(stdinFile.Fd()), state)
		}()

		windowChange := devssh.WatchWindowSize(ctx)
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case <-windowChange:
				}
				width, height, err := term.GetSize(int(stdoutFile.Fd()))
				if err != nil {
					continue
				}
				_ = session.WindowChange(height, width)
			}
		}()

		err = session.RequestPty("xterm-256color", 128, 128, ssh.TerminalModes{})
		if err != nil {
			return err
		}
	}

	session.Stdin = stdin
	session.Stdout = stdout
	session.Stderr = stderr
	err = session.Shell()
	if err != nil {
		return err
	}

	// set correct window size
	if validOut && validIn && isatty.IsTerminal(stdoutFile.Fd()) {
		width, height, err := term.GetSize(int(stdoutFile.Fd()))
		if err == nil {
			_ = session.WindowChange(height, width)
		}
	}

	// wait until done
	err = session.Wait()
	if err != nil {
		return err
	}

	return nil
}

func (cmd *SSHCmd) jumpContainer(ctx context.Context, client *client.WorkspaceClient) error {
	// lock workspace
	unlockOnce := sync.Once{}
	err := client.Lock(ctx)
	if err != nil {
		return err
	}
	defer unlockOnce.Do(client.Unlock)

	// ensure pod running
	err = ensureRunning(ctx, client)
	if err != nil {
		return err
	}

	writer := client.Log.ErrorStreamOnly().Writer(logrus.InfoLevel, false)
	defer writer.Close()

	cancelCtx, cancel := context.WithCancel(ctx)

	// make many pipe
	defer cancel()
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		return err
	}
	stdinReader, stdinWriter, err := os.Pipe()
	if err != nil {
		return err
	}
	defer stdoutWriter.Close()
	defer stdinWriter.Close()

	// start ssh-server on the remote
	tunnelChan := make(chan error, 1)
	stderr := log.Default.ErrorStreamOnly().Writer(logrus.InfoLevel, false)
	defer stderr.Close()
	go func() {
		defer client.Log.Infof("tunnel to host closed")

		tunnelChan <- kubernetes.Exec(cancelCtx, cmd.NameSpace, cmd.Service, stdinReader, stdoutWriter, stderr)
	}()

	containerChan := make(chan error, 1)
	go func() {
		// start ssh client as target user
		sshClient, err := devssh.StdioClientWithUser(stdoutReader, stdinWriter, cmd.User, false)
		// sshClient, err := devssh.StdioClient(stdoutReader, stdinWriter, false)
		if err != nil {
			containerChan <- errors.Wrap(err, "create ssh client")
			return
		}
		defer sshClient.Close()
		defer cancel()
		defer client.Log.Infof("Connection to container closed")
		client.Log.Infof("Successfully connected to host")
		unlockOnce.Do(client.Unlock)
		containerChan <- errors.Wrap(cmd.startService(cancelCtx, sshClient, stderr), "run in container")
	}()
	select {
	case err := <-containerChan:
		return errors.Wrap(err, "tunnel to container")
	case err := <-tunnelChan:
		return errors.Wrap(err, "connect to server")
	}
}
