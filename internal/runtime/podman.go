package runtime

// NewPodmanRuntime creates a Runtime that connects to a Podman daemon via
// its Docker-compatible socket API. The DOCKER_HOST env var should point to
// the Podman socket (e.g. unix:///run/user/1000/podman/podman.sock).
func NewPodmanRuntime() (*DockerRuntime, error) {
	// Podman exposes a Docker-compatible REST API, so the Docker SDK works as-is.
	return NewDockerRuntime()
}
