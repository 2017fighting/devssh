package client

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/2017fighting/devssh/pkg/kubernetes"
	"github.com/2017fighting/devssh/pkg/provider"
	"github.com/gofrs/flock"
	"github.com/loft-sh/devpod/pkg/client"
	"github.com/loft-sh/log"
	"k8s.io/apimachinery/pkg/api/errors"
)

type WorkspaceClient struct {
	m        sync.Mutex
	lockOnce sync.Once
	lock     *flock.Flock

	Service   string
	Namespace string
	Log       log.Logger
}

func NewWorkspaceClient(namespace string, service string, log log.Logger) *WorkspaceClient {
	return &WorkspaceClient{
		Namespace: namespace,
		Service:   service,
		Log:       log,
	}
}

func printLogMessagePeriodically(message string, log log.Logger) chan struct{} {
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			case <-time.After(time.Second * 5):
				log.Info(message)
			}
		}
	}()

	return done
}

func tryLock(ctx context.Context, lock *flock.Flock, name string, log log.Logger) error {
	done := printLogMessagePeriodically(fmt.Sprintf("Trying to lock %s, seems like another process is running that blocks this %s", name, name), log)
	defer close(done)

	now := time.Now()
	for time.Since(now) < time.Minute*5 {
		locked, err := lock.TryLock()
		if err != nil {
			return err
		} else if locked {
			return nil
		}

		select {
		case <-time.After(time.Second):
			continue
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return fmt.Errorf("timed out waiting to lock %s, seems like there is another process running on this machine that blocks it", name)
}

func (s *WorkspaceClient) initLock() {
	s.lockOnce.Do(func() {
		s.m.Lock()
		defer s.m.Unlock()

		// get locks dir
		workspaceLockDir, err := provider.GetLocksDir(s.Service)
		if err != nil {
			panic(fmt.Errorf("get lock dir: %w", err))
		}
		_ = os.MkdirAll(workspaceLockDir, 0777)

		// create workspace lock
		s.lock = flock.New(filepath.Join(workspaceLockDir, "workspace.lock"))
	})

}
func (s *WorkspaceClient) Lock(ctx context.Context) error {
	s.initLock()
	s.Log.Debugf("Acquire lock...")
	err := tryLock(ctx, s.lock, "workspace", s.Log)
	if err != nil {
		return fmt.Errorf("error locking workspace: %w", err)
	}
	s.Log.Debugf("Acquired workspace lock...")
	return nil
}

func (s *WorkspaceClient) Unlock() {
	s.initLock()

	err := s.lock.Unlock()
	if err != nil {
		s.Log.Warnf("Error unlocking workspace: %v", err)
	}
}

func (s *WorkspaceClient) Status(ctx context.Context) (client.Status, error) {
	err := kubernetes.IsSVCRunning(s.Namespace, s.Service)
	if errors.IsNotFound(err) {
		return client.StatusNotFound, nil
	} else if statusError, isStatus := err.(*errors.StatusError); isStatus {
		panic(fmt.Errorf("get svc in k8s: %v", statusError.ErrStatus.Message))
	} else if err != nil {
		panic(fmt.Errorf("get svc in k8s: %w", err))
	}
	log.Default.Info("running")
	return client.StatusRunning, nil
}
