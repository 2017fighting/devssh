package tunnelserver

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"

	"github.com/go-logr/logr"
	"github.com/loft-sh/devpod/pkg/agent/tunnel"
	"github.com/loft-sh/devpod/pkg/devcontainer/config"
	"github.com/loft-sh/devpod/pkg/netstat"
	provider2 "github.com/loft-sh/devpod/pkg/provider"
	"github.com/loft-sh/devpod/pkg/stdio"
	"github.com/loft-sh/log"
	"github.com/loft-sh/log/scanner"
	"github.com/loft-sh/log/survey"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/resolver"
)

type tunnelServer struct {
	tunnel.UnimplementedTunnelServer

	// stream mounts
	mounts []*config.Mount

	forwarder              netstat.Forwarder
	allowGitCredentials    bool
	allowDockerCredentials bool
	result                 *config.Result
	workspace              *provider2.Workspace
	log                    log.Logger
	gitCredentialsOverride gitCredentialsOverride
}

type gitCredentialsOverride struct {
	username string
	token    string
}

func NewTunnelClient(reader io.Reader, writer io.WriteCloser, exitOnClose bool, exitCode int) (tunnel.TunnelClient, error) {
	pipe := stdio.NewStdioStream(reader, writer, exitOnClose, exitCode)

	// After moving from deprecated grpc.Dial to grpc.NewClient we need to setup resolver first
	// https://github.com/grpc/grpc-go/issues/1786#issuecomment-2119088770
	resolver.SetDefaultScheme("passthrough")

	// Set up a connection to the server.
	conn, err := grpc.NewClient("",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
			return pipe, nil
		}),
	)
	if err != nil {
		return nil, err
	}

	c := tunnel.NewTunnelClient(conn)

	return c, nil
}

func NewTunnelLogger(ctx context.Context, client tunnel.TunnelClient, debug bool) log.Logger {
	level := logrus.InfoLevel
	if debug {
		level = logrus.DebugLevel
	}

	return &tunnelLogger{ctx: ctx, client: client, level: level}
}

type tunnelLogger struct {
	ctx    context.Context
	level  logrus.Level
	client tunnel.TunnelClient
}

func (s *tunnelLogger) Debug(args ...interface{}) {
	if s.level < logrus.DebugLevel {
		return
	}

	_, _ = s.client.Log(s.ctx, &tunnel.LogMessage{
		LogLevel: tunnel.LogLevel_DEBUG,
		Message:  fmt.Sprintln(args...),
	})
}

func (s *tunnelLogger) Debugf(format string, args ...interface{}) {
	if s.level < logrus.DebugLevel {
		return
	}

	_, _ = s.client.Log(s.ctx, &tunnel.LogMessage{
		LogLevel: tunnel.LogLevel_DEBUG,
		Message:  fmt.Sprintf(format, args...) + "\n",
	})
}

func (s *tunnelLogger) Info(args ...interface{}) {
	if s.level < logrus.InfoLevel {
		return
	}

	_, _ = s.client.Log(s.ctx, &tunnel.LogMessage{
		LogLevel: tunnel.LogLevel_INFO,
		Message:  fmt.Sprintln(args...),
	})
}

func (s *tunnelLogger) Infof(format string, args ...interface{}) {
	if s.level < logrus.InfoLevel {
		return
	}

	_, _ = s.client.Log(s.ctx, &tunnel.LogMessage{
		LogLevel: tunnel.LogLevel_INFO,
		Message:  fmt.Sprintf(format, args...) + "\n",
	})
}

func (s *tunnelLogger) Warn(args ...interface{}) {
	if s.level < logrus.WarnLevel {
		return
	}

	_, _ = s.client.Log(s.ctx, &tunnel.LogMessage{
		LogLevel: tunnel.LogLevel_WARNING,
		Message:  fmt.Sprintln(args...),
	})
}

func (s *tunnelLogger) Warnf(format string, args ...interface{}) {
	if s.level < logrus.WarnLevel {
		return
	}

	_, _ = s.client.Log(s.ctx, &tunnel.LogMessage{
		LogLevel: tunnel.LogLevel_WARNING,
		Message:  fmt.Sprintf(format, args...) + "\n",
	})
}

func (s *tunnelLogger) Error(args ...interface{}) {
	if s.level < logrus.ErrorLevel {
		return
	}

	_, _ = s.client.Log(s.ctx, &tunnel.LogMessage{
		LogLevel: tunnel.LogLevel_ERROR,
		Message:  fmt.Sprintln(args...),
	})
}

func (s *tunnelLogger) Errorf(format string, args ...interface{}) {
	if s.level < logrus.ErrorLevel {
		return
	}

	_, _ = s.client.Log(s.ctx, &tunnel.LogMessage{
		LogLevel: tunnel.LogLevel_ERROR,
		Message:  fmt.Sprintf(format, args...) + "\n",
	})
}

func (s *tunnelLogger) Fatal(args ...interface{}) {
	if s.level < logrus.FatalLevel {
		return
	}

	_, _ = s.client.Log(s.ctx, &tunnel.LogMessage{
		LogLevel: tunnel.LogLevel_ERROR,
		Message:  fmt.Sprintln(args...),
	})

	os.Exit(1)
}

func (s *tunnelLogger) Fatalf(format string, args ...interface{}) {
	if s.level < logrus.FatalLevel {
		return
	}

	_, _ = s.client.Log(s.ctx, &tunnel.LogMessage{
		LogLevel: tunnel.LogLevel_ERROR,
		Message:  fmt.Sprintf(format, args...) + "\n",
	})

	os.Exit(1)
}

func (s *tunnelLogger) Done(args ...interface{}) {
	if s.level < logrus.InfoLevel {
		return
	}

	_, _ = s.client.Log(s.ctx, &tunnel.LogMessage{
		LogLevel: tunnel.LogLevel_DONE,
		Message:  fmt.Sprintln(args...),
	})
}

func (s *tunnelLogger) Donef(format string, args ...interface{}) {
	if s.level < logrus.InfoLevel {
		return
	}

	_, _ = s.client.Log(s.ctx, &tunnel.LogMessage{
		LogLevel: tunnel.LogLevel_DONE,
		Message:  fmt.Sprintf(format, args...) + "\n",
	})
}

func (s *tunnelLogger) Print(level logrus.Level, args ...interface{}) {
	switch level {
	case logrus.InfoLevel:
		s.Info(args...)
	case logrus.DebugLevel:
		s.Debug(args...)
	case logrus.WarnLevel:
		s.Warn(args...)
	case logrus.ErrorLevel:
		s.Error(args...)
	case logrus.FatalLevel:
		s.Fatal(args...)
	case logrus.PanicLevel:
		s.Fatal(args...)
	case logrus.TraceLevel:
		s.Debug(args...)
	}
}

func (s *tunnelLogger) Printf(level logrus.Level, format string, args ...interface{}) {
	switch level {
	case logrus.InfoLevel:
		s.Infof(format, args...)
	case logrus.DebugLevel:
		s.Debugf(format, args...)
	case logrus.WarnLevel:
		s.Warnf(format, args...)
	case logrus.ErrorLevel:
		s.Errorf(format, args...)
	case logrus.FatalLevel:
		s.Fatalf(format, args...)
	case logrus.PanicLevel:
		s.Fatalf(format, args...)
	case logrus.TraceLevel:
		s.Debugf(format, args...)
	}
}

func (s *tunnelLogger) SetLevel(level logrus.Level) {
	s.level = level
}

func (s *tunnelLogger) GetLevel() logrus.Level {
	return s.level
}

func (s *tunnelLogger) Writer(level logrus.Level, raw bool) io.WriteCloser {
	if s.level < level {
		return &log.NopCloser{Writer: io.Discard}
	}

	reader, writer := io.Pipe()
	go func() {
		sa := scanner.NewScanner(reader)
		for sa.Scan() {
			if raw {
				s.WriteString(level, sa.Text()+"\n")
			} else {
				s.Print(level, sa.Text())
			}
		}
	}()

	return writer
}

func (s *tunnelLogger) WriteString(level logrus.Level, message string) {
	if s.level < level {
		return
	}

	// TODO: support this correctly
	s.Print(level, message)
}

func (s *tunnelLogger) Question(params *survey.QuestionOptions) (string, error) {
	return "", fmt.Errorf("not supported")
}

func (s *tunnelLogger) ErrorStreamOnly() log.Logger {
	return s
}

func (*tunnelLogger) LogrLogSink() logr.LogSink {
	return nil
}

func RunServicesServer(ctx context.Context, reader io.Reader, writer io.WriteCloser, allowGitCredentials, allowDockerCredentials bool, forwarder netstat.Forwarder, log log.Logger, options ...Option) error {
	opts := append(options, []Option{
		WithForwarder(forwarder),
		WithAllowGitCredentials(allowGitCredentials),
		WithAllowDockerCredentials(allowDockerCredentials),
	}...)
	tunnelServ := New(log, opts...)

	return tunnelServ.Run(ctx, reader, writer)
}

func (t *tunnelServer) RunWithResult(ctx context.Context, reader io.Reader, writer io.WriteCloser) (*config.Result, error) {
	lis := stdio.NewStdioListener(reader, writer, false)
	s := grpc.NewServer()
	tunnel.RegisterTunnelServer(s, t)
	reflection.Register(s)
	errChan := make(chan error, 1)
	go func() {
		errChan <- s.Serve(lis)
	}()

	select {
	case err := <-errChan:
		return nil, err
	case <-ctx.Done():
		return t.result, nil
	}
}

func (t *tunnelServer) Run(ctx context.Context, reader io.Reader, writer io.WriteCloser) error {
	_, err := t.RunWithResult(ctx, reader, writer)
	return err
}

func New(log log.Logger, options ...Option) *tunnelServer {
	s := &tunnelServer{
		log: log,
	}
	for _, o := range options {
		s = o(s)
	}

	return s
}
