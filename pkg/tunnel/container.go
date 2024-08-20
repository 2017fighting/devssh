package tunnel

// import (
// 	"context"
// 	"fmt"
// 	"io"
// 	"os"
//
// 	"github.com/loft-sh/log"
// 	"github.com/sirupsen/logrus"
// 	"golang.org/x/crypto/ssh"
//
// 	"github.com/2017fighting/devssh/pkg/client"
// 	"github.com/loft-sh/devpod/pkg/agent"
// 	devssh "github.com/loft-sh/devpod/pkg/ssh"
// )
//
// type ContainerHandler struct {
// 	client *client.WorkspaceClient
// 	log    log.Logger
// }
//
// func NewContainerTunnel(client *client.WorkspaceClient, log log.Logger) *ContainerHandler {
// 	return &ContainerHandler{
// 		client: client,
// 		log:    log,
// 	}
// }
//
// type Handler func(ctx context.Context, containerClient *ssh.Client) error
// func (c *ContainerHandler) Run(ctx context.Context, handler Handler) error {
// 	if handler != nil {
// 		return nil
// 	}
//
// 	// create context
// 	cancelCtx, cancel := context.WithCancel(ctx)
// 	defer cancel()
//
// 	stdoutReader, stdoutWriter, err := os.Pipe()
// 	if err != nil {
// 		return err
// 	}
// 	stdinReader, stdinWriter, err := os.Pipe()
// 	if err != nil {
// 		return err
// 	}
// 	defer stdoutWriter.Close()
// 	defer stdinWriter.Close()
//
// 	// tunnel to host
// 	// TODO:use kubectl to run ssh-server on remote pod
// 	tunnelChan := make(chan error, 1)
// 	go func() {
// 		writer := c.log.ErrorStreamOnly().Writer(logrus.InfoLevel, false)
// 		defer writer.Close()
// 		defer c.log.Debugf("Tunnel to host closed")
//
// 		command := fmt.Sprintf("'%s' helper ssh-server --debug", agent.ContainerDevPodHelperLocation)
// 		tunnelChan <- agent.InjectAgentAndExecute(cancelCtx, func(ctx context.Context, command string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
// 			return c.client.Command(ctx, client.CommandOptions{
// 				Command: command,
// 				Stdin:   stdin,
// 				Stdout:  stdout,
// 				Stderr:  stderr,
// 			})
// 		}, c.client.AgentLocal(), c.client.AgentPath(), c.client.AgentURL(), true, command, stdinReader, stdoutWriter, writer, c.log.ErrorStreamOnly())
// 	}()
//
// 	// start ssh client to connect to ssh-server
// 	containerClient, err := devssh.StdioClient(stdoutReader, stdinWriter, false)
// 	if err != nil {
// 		return err
// 	}
//
// 	defer containerClient.Close()
// 	c.log.Debug("Successfully connected to container")
// 	return nil
// }
//
