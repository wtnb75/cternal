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
	"github.com/docker/docker/pkg/stdcopy"
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

	cols, rows := opts.Cols, opts.Rows
	if cols == 0 {
		cols = 80
	}
	if rows == 0 {
		rows = 24
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

	_ = d.cli.ContainerExecResize(ctx, execResp.ID, container.ResizeOptions{
		Height: uint(rows),
		Width:  uint(cols),
	})

	// Tty=true: Docker daemon sends raw PTY bytes (no multiplex framing).
	ds := newDockerStream(resp, true)
	ds.execID = execResp.ID
	ds.cli = d.cli
	return ds, nil
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
	// ContainerAttach without a Tty container uses Docker's multiplexed framing.
	// Use stdcopy to demultiplex; this also works correctly for Tty containers
	// because stdcopy merges stdout+stderr into a single stream.
	ds := newDockerStream(resp, false)
	ds.containerID = id
	ds.cli = d.cli
	return ds, nil
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
		Timestamps: opts.Timestamps,
	})
	if err != nil {
		return nil, fmt.Errorf("logs: %w", err)
	}
	// ContainerLogs always uses Docker's multiplexed framing (even without Tty).
	// Pipe stdout+stderr through stdcopy so callers receive plain text.
	pr, pw := io.Pipe()
	go func() {
		_, _ = stdcopy.StdCopy(pw, pw, rc)
		_ = pw.Close()
		_ = rc.Close()
	}()
	return pr, nil
}

// dockerStream wraps a Docker HijackedResponse as a Stream.
// When tty=true the daemon sends raw PTY bytes; when false it uses Docker's
// 8-byte multiplexed framing (stdcopy format).  newDockerStream sets up an
// io.Pipe + stdcopy.StdCopy goroutine for the non-TTY case so that Read()
// always returns clean data regardless of the underlying framing.
type dockerStream struct {
	conn        dockertypes.HijackedResponse
	execID      string
	containerID string
	cli         *client.Client
	reader      io.Reader // raw conn.Reader (tty) or demuxed pipe (non-tty)
}

// newDockerStream builds a dockerStream.  pass tty=true when the exec/attach
// was created with Tty:true (raw PTY stream); pass tty=false for multiplexed
// streams (attach without Tty, logs).
func newDockerStream(conn dockertypes.HijackedResponse, tty bool) *dockerStream {
	var r io.Reader
	if tty {
		r = conn.Reader
	} else {
		pr, pw := io.Pipe()
		go func() {
			_, _ = stdcopy.StdCopy(pw, pw, conn.Reader)
			_ = pw.Close()
		}()
		r = pr
	}
	return &dockerStream{conn: conn, reader: r}
}

func (s *dockerStream) Read() ([]byte, error) {
	buf := make([]byte, 4096)
	n, err := s.reader.Read(buf)
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
