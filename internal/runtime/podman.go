package runtime

import "github.com/docker/docker/client"

// NewPodmanRuntime creates a Runtime that connects to a Podman daemon via its
// Docker-compatible socket API.  If host is empty, DOCKER_HOST env var is used
// (e.g. unix:///run/user/1000/podman/podman.sock).
func NewPodmanRuntime(host string) (*DockerRuntime, error) {
	opts := []client.Opt{client.FromEnv, client.WithAPIVersionNegotiation()}
	if host != "" {
		opts = append(opts, client.WithHost(host))
	}
	cli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, err
	}
	return &DockerRuntime{cli: cli}, nil
}
