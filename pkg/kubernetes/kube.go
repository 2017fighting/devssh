package kubernetes

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/2017fighting/devssh/pkg/agent"
	"github.com/loft-sh/log"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/util/homedir"
)

func getK8sClient() (*restclient.Config, *kubernetes.Clientset) {
	kubeconfigPath := filepath.Join(homedir.HomeDir(), ".kube", "config")

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		panic(fmt.Errorf("build k8s config: %w", err))
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(fmt.Errorf("new k8s client: %w", err))
	}
	return config, clientset
}

func IsSVCRunning(namespace string, service string) error {
	_, clientset := getK8sClient()
	_, err := clientset.CoreV1().Services(namespace).Get(context.TODO(), service, metav1.GetOptions{})
	return err
}

func getPodByService(namespace string, service string) string {
	_, clientset := getK8sClient()
	svc, err := clientset.CoreV1().Services(namespace).Get(context.TODO(), service, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		panic(fmt.Errorf("svc(%v) not found in namespace:%v", service, namespace))
	} else if statusError, isStatus := err.(*errors.StatusError); isStatus {
		panic(fmt.Errorf("get svc in k8s: %v", statusError.ErrStatus.Message))
	} else if err != nil {
		panic(fmt.Errorf("get svc in k8s: %w", err))
	}
	pods, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: labels.Set(svc.Spec.Selector).AsSelector().String(),
	})
	if err != nil {
		panic(fmt.Errorf("get pods: %w", err))
	}
	for _, pod := range pods.Items {
		return pod.Name
	}
	return ""
}

func Exec(ctx context.Context, namespace string, service string, workdir string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	config, clientset := getK8sClient()
	podName := getPodByService(namespace, service)
	log.Default.Infof("get podName:%v", podName)
	req := clientset.CoreV1().RESTClient().Post().Resource("pods").Namespace(namespace).Name(podName).SubResource("exec").VersionedParams(
		&corev1.PodExecOptions{
			Command: []string{agent.ContainerDevPodHelperLocation, "ssh-server", "--workdir", workdir},
			// Command: []string{"ls", "/mnt"},
			// Command: []string{"zsh"},
			Stdin:  true,
			Stdout: true,
			Stderr: true,
			TTY:    false,
			// TTY:    true,
		}, scheme.ParameterCodec,
	)

	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return fmt.Errorf("k8s remote exec: %s", err)
	}
	// screen := struct {
	// 	io.Reader
	// 	io.Writer
	// }{os.Stdin, os.Stdout}

	if err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  stdin,
		Stdout: stdout,
		// Stdin:  screen,
		// Stdout: screen,
		Stderr: stderr,
		Tty:    true,
	}); err != nil {
		return fmt.Errorf("k8s exec: %w", err)
	}
	return nil
}
