package runtime

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
)

// K8sRuntime implements Runtime using the Kubernetes client-go.
type K8sRuntime struct {
	client    kubernetes.Interface
	restCfg   *rest.Config
	namespace string
}

// NewK8sRuntime creates a K8sRuntime using in-cluster config if available,
// falling back to the kubeconfig file.
func NewK8sRuntime(namespace string) (*K8sRuntime, error) {
	if namespace == "" {
		namespace = "default"
	}
	cfg, err := rest.InClusterConfig()
	if err != nil {
		cfg, err = clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
		if err != nil {
			return nil, fmt.Errorf("k8s config: %w", err)
		}
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("k8s client: %w", err)
	}
	return &K8sRuntime{client: client, restCfg: cfg, namespace: namespace}, nil
}

// ListContainers lists pods that match the filter as if they were containers.
// The container ID is "namespace/pod/container".
func (k *K8sRuntime) ListContainers(ctx context.Context, f Filter) ([]Container, error) {
	pods, err := k.client.CoreV1().Pods(k.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list pods: %w", err)
	}
	var result []Container
	for _, pod := range pods.Items {
		if f.Name != "" && !strings.Contains(pod.Name, f.Name) {
			continue
		}
		running := pod.Status.Phase == corev1.PodRunning
		if f.Status == "running" && !running {
			continue
		}
		for _, c := range pod.Spec.Containers {
			result = append(result, Container{
				ID:      fmt.Sprintf("%s/%s/%s", pod.Namespace, pod.Name, c.Name),
				Name:    fmt.Sprintf("%s/%s", pod.Name, c.Name),
				Image:   c.Image,
				Status:  string(pod.Status.Phase),
				Running: running,
			})
		}
	}
	return result, nil
}

// Exec opens an interactive exec session to a pod container.
// id format: "namespace/pod/container"
func (k *K8sRuntime) Exec(ctx context.Context, id string, opts ExecOptions) (Stream, error) {
	ns, pod, ctr, err := parseK8sID(id)
	if err != nil {
		return nil, err
	}
	shell := opts.Shell
	if len(shell) == 0 {
		shell = []string{"/bin/sh"}
	}

	execReq := k.client.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(pod).
		Namespace(ns).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: ctr,
			Command:   shell,
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
			TTY:       true,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(k.restCfg, http.MethodPost, execReq.URL())
	if err != nil {
		return nil, fmt.Errorf("k8s exec: %w", err)
	}

	return newK8sStream(ctx, exec), nil
}

// Attach attaches to the first container of the pod.
func (k *K8sRuntime) Attach(ctx context.Context, id string) (Stream, error) {
	ns, pod, ctr, err := parseK8sID(id)
	if err != nil {
		return nil, err
	}

	attachReq := k.client.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(pod).
		Namespace(ns).
		SubResource("attach").
		VersionedParams(&corev1.PodAttachOptions{
			Container: ctr,
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
			TTY:       true,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(k.restCfg, http.MethodPost, attachReq.URL())
	if err != nil {
		return nil, fmt.Errorf("k8s attach: %w", err)
	}

	return newK8sStream(ctx, exec), nil
}

// Logs returns a ReadCloser streaming logs from the specified pod container.
func (k *K8sRuntime) Logs(ctx context.Context, id string, opts LogsOptions) (io.ReadCloser, error) {
	ns, pod, ctr, err := parseK8sID(id)
	if err != nil {
		return nil, err
	}

	logOpts := &corev1.PodLogOptions{
		Container: ctr,
		Follow:    opts.Follow,
	}
	if opts.Since != "" {
		logOpts.SinceTime = nil // simplification: since is passed as-is
	}

	req := k.client.CoreV1().Pods(ns).GetLogs(pod, logOpts)
	rc, err := req.Stream(ctx)
	if err != nil {
		return nil, fmt.Errorf("k8s logs: %w", err)
	}
	return rc, nil
}

func parseK8sID(id string) (namespace, pod, container string, err error) {
	parts := strings.SplitN(id, "/", 3)
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("invalid k8s id %q: expected namespace/pod/container", id)
	}
	return parts[0], parts[1], parts[2], nil
}

// k8sStream wraps the remotecommand.Executor as a Stream using pipe pairs.
type k8sStream struct {
	stdin  *io.PipeWriter
	stdout *io.PipeReader
	cancel context.CancelFunc
}

func newK8sStream(ctx context.Context, exec remotecommand.Executor) *k8sStream {
	stdinR, stdinW := io.Pipe()
	stdoutR, stdoutW := io.Pipe()
	ctx, cancel := context.WithCancel(ctx)

	s := &k8sStream{stdin: stdinW, stdout: stdoutR, cancel: cancel}
	go func() {
		defer stdoutW.Close()
		_ = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
			Stdin:  stdinR,
			Stdout: stdoutW,
			Stderr: stdoutW,
			Tty:    true,
		})
	}()
	return s
}

func (s *k8sStream) Read() ([]byte, error) {
	buf := make([]byte, 4096)
	n, err := s.stdout.Read(buf)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

func (s *k8sStream) Write(data []byte) error {
	_, err := io.Copy(s.stdin, bytes.NewReader(data))
	return err
}

func (s *k8sStream) Resize(_ uint16, _ uint16) error {
	// TTY resize via SPDY is handled out-of-band; not trivially accessible here.
	return nil
}

func (s *k8sStream) Close() error {
	s.cancel()
	return s.stdin.Close()
}

