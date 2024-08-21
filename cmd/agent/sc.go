package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"

	"github.com/2017fighting/devssh/pkg/agent/tunnelserver"
	"github.com/loft-sh/devpod/pkg/agent/tunnel"
	"github.com/loft-sh/devpod/pkg/credentials"
	"github.com/loft-sh/devpod/pkg/gitcredentials"
	portpkg "github.com/loft-sh/devpod/pkg/port"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

const ExitCodeIO int = 64

type CSCmd struct {
	User string
}

func NewCSCmd() *cobra.Command {
	cmd := &CSCmd{}
	csCmd := &cobra.Command{
		Use:   "credentials-server",
		Short: "start credentials server",
		RunE: func(_ *cobra.Command, args []string) error {
			return cmd.Run(context.Background())
		},
	}
	csCmd.Flags().StringVar(&cmd.User, "user", "root", "change this user's config")
	return csCmd
}

func (cmd *CSCmd) Run(ctx context.Context) error {
	// create a grpc client
	tunnelClient, err := tunnelserver.NewTunnelClient(os.Stdin, os.Stdout, true, ExitCodeIO)
	if err != nil {
		return fmt.Errorf("error creating tunnel client: %w", err)
	}

	// this message serves as a ping to the client
	_, err = tunnelClient.Ping(ctx, &tunnel.Empty{})
	if err != nil {
		return errors.Wrap(err, "ping client")
	}

	// create debug logger
	log := tunnelserver.NewTunnelLogger(ctx, tunnelClient, true)
	log.Debugf("Start credentials server")

	port, err := credentials.GetPort()
	if err != nil {
		return err
	}

	//check local port
	addr := net.JoinHostPort("localhost", strconv.Itoa(port))
	if ok, err := portpkg.IsAvailable(addr); !ok || err != nil {
		log.Debugf("Port %d not available, exiting", port)
		return nil
	}
	//TODO: configure docker credential helper

	binaryPath, err := os.Executable()
	log.Debugf(binaryPath)

	if err != nil {
		return err
	}
	// configure git user
	err = configureGitUserLocally(ctx, cmd.User, tunnelClient)
	if err != nil {
		log.Debugf("Error configuring git user: %v", err)
	}

	// configure git credential helper
	err = gitcredentials.ConfigureHelper(binaryPath, cmd.User, port)
	if err != nil {
		return errors.Wrap(err, "configure git helper")
	}

	// cleanup when we are done
	defer func(userName string) {
		_ = gitcredentials.RemoveHelper(userName)
	}(cmd.User)

	return credentials.RunCredentialsServer(ctx, port, tunnelClient, log)
}

func configureGitUserLocally(ctx context.Context, userName string, client tunnel.TunnelClient) error {
	// get local credentials
	localGitUser, err := gitcredentials.GetUser(userName)
	if err != nil {
		return err
	} else if localGitUser.Name != "" && localGitUser.Email != "" {
		return nil
	}

	// set user & email if not found
	response, err := client.GitUser(ctx, &tunnel.Empty{})
	if err != nil {
		return fmt.Errorf("retrieve git user: %w", err)
	}

	// parse git user from response
	gitUser := &gitcredentials.GitUser{}
	err = json.Unmarshal([]byte(response.Message), gitUser)
	if err != nil {
		return fmt.Errorf("decode git user: %w", err)
	}

	// don't override what is already there
	if localGitUser.Name != "" {
		gitUser.Name = ""
	}
	if localGitUser.Email != "" {
		gitUser.Email = ""
	}

	// set git user
	err = gitcredentials.SetUser(userName, gitUser)
	if err != nil {
		return fmt.Errorf("set git user & email: %w", err)
	}

	return nil
}
