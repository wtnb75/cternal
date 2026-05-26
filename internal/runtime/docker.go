package runtime

import (
	"context"
	"fmt"
	"io"
	"strings"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

// DockerRuntime implements Runtime using the Docker SDK.
type DockerRuntime struct {
	cli *client.Client
}

// NewDockerRuntime creates a DockerRuntime connected to the Docker daemon
// described by the standard DOCKER_HOST / DOCKER_TLS_VERIFY environment variables.
func NewDockerRuntime() (*DockerRuntime, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("docker client: %w", err)
	}
	return &DockerRuntime{cli: cli}, nil
}

func (d *DockerRuntime) ListContainers(ctx context.Context, f Filter) ([]Container, error) {
	args := filters.NewArgs()
	if f.Name != "" {
		args.Add("name", f.Name)
	}
	if f.Status != "" {
		args.Add("status", f.Status)
	}
	for k, v := range f.Labels {
		args.Add("label", k+"="+v)
	}

	list, err := d.cli.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: args,
	})
	if err != nil {
		return nil, fmt.Errorf("list containers: %w", err)
	}

	result := make([]Container, 0, len(list))
	for _, c := range list {
		name := ""
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}
		result = append(result, Container{
			ID:      c.ID,
			Name:    name,
			Image:   c.Image,
			Status:  c.Status,
			Running: c.State == "running",
		})
	}
	return result, nil
}

func (d *DockerRuntime) Exec(ctx context.Context, id string, opts ExecOptions) (Stream, error) {
	shell := opts.Shell
	if len(shell) == 0 {
		shell = []string{"/bin/sh"}
	}

	execResp, err := d.cli.ContainerExecCreate(ctx, id, container.ExecOptions{
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
		Cmd:          shell,
		Env:          opts.Env,
	})
	if err != nil {
		return nil, fmt.Errorf("exec create: %w", err)
	}

	resp, err := d.cli.ContainerExecAttach(ctx, execResp.ID, container.ExecAttachOptions{Tty: true})
	if err != nil {
		return nil, fmt.Errorf("exec attach: %w", err)
	}

	if opts.Cols > 0 || opts.Rows > 0 {
		_ = d.cli.ContainerExecResize(ctx, execResp.ID, container.ResizeOptions{
			Height: uint(opts.Rows),
			Width:  uint(opts.Cols),
		})
	}

	return &dockerStream{
		conn:   resp,
		execID: execResp.ID,
		cli:    d.cli,
	}, nil
}

func (d *DockerRuntime) Attach(ctx context.Context, id string) (Stream, error) {
	resp, err := d.cli.ContainerAttach(ctx, id, container.AttachOptions{
		Stream: true,
		Stdin:  true,
		Stdout: true,
		Stderr: true,
	})
	if err != nil {
		return nil, fmt.Errorf("attach: %w", err)
	}
	return &dockerStream{conn: resp, cli: d.cli, containerID: id}, nil
}

func (d *DockerRuntime) Logs(ctx context.Context, id string, opts LogsOptions) (io.ReadCloser, error) {
	since := opts.Since
	if since == "" {
		// Default: logs from container start
		info, err := d.cli.ContainerInspect(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("inspect container: %w", err)
		}
		since = info.State.StartedAt
	}

	rc, err := d.cli.ContainerLogs(ctx, id, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     opts.Follow,
		Since:      since,
	})
	if err != nil {
		return nil, fmt.Errorf("logs: %w", err)
	}
	return rc, nil
}

// dockerStream wraps a Docker HijackedResponse as a Stream.
type dockerStream struct {
	conn        dockertypes.HijackedResponse
	execID      string
	containerID string
	cli         *client.Client
}

func (s *dockerStream) Read() ([]byte, error) {
	buf := make([]byte, 4096)
	n, err := s.conn.Reader.Read(buf)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

func (s *dockerStream) Write(data []byte) error {
	_, err := s.conn.Conn.Write(data)
	return err
}

func (s *dockerStream) Resize(cols, rows uint16) error {
	ctx := context.Background()
	if s.execID != "" {
		return s.cli.ContainerExecResize(ctx, s.execID, container.ResizeOptions{
			Width:  uint(cols),
			Height: uint(rows),
		})
	}
	if s.containerID != "" {
		return s.cli.ContainerResize(ctx, s.containerID, container.ResizeOptions{
			Width:  uint(cols),
			Height: uint(rows),
		})
	}
	return nil
}

func (s *dockerStream) Close() error {
	s.conn.Close()
	return nil
}
