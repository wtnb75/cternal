package runtime

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/client"
)

// DockerRuntime implements Runtime using the Docker SDK.
type DockerRuntime struct {
	cli *client.Client
}

// NewDockerRuntime creates a DockerRuntime connected to the Docker daemon
// described by the standard DOCKER_HOST / DOCKER_TLS_VERIFY environment variables.
func NewDockerRuntime() (*DockerRuntime, error) {
	cli, err := client.New(client.FromEnv)
	if err != nil {
		return nil, fmt.Errorf("docker client: %w", err)
	}
	return &DockerRuntime{cli: cli}, nil
}

func (d *DockerRuntime) ListContainers(ctx context.Context, f Filter) ([]Container, error) {
	args := make(client.Filters)
	if f.Name != "" {
		args.Add("name", f.Name)
	}
	if f.Status != "" {
		args.Add("status", f.Status)
	}
	for k, v := range f.Labels {
		args.Add("label", k+"="+v)
	}

	list, err := d.cli.ContainerList(ctx, client.ContainerListOptions{
		All:     true,
		Filters: args,
	})
	if err != nil {
		return nil, fmt.Errorf("list containers: %w", err)
	}

	result := make([]Container, 0, len(list.Items))
	for _, c := range list.Items {
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
			Runtime: "docker",
			Labels:  c.Labels,
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

	execResp, err := d.cli.ExecCreate(ctx, id, client.ExecCreateOptions{
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		TTY:          true,
		Cmd:          shell,
		Env:          opts.Env,
	})
	if err != nil {
		return nil, fmt.Errorf("exec create: %w", err)
	}

	resp, err := d.cli.ExecAttach(ctx, execResp.ID, client.ExecAttachOptions{TTY: true})
	if err != nil {
		return nil, fmt.Errorf("exec attach: %w", err)
	}

	_, _ = d.cli.ExecResize(ctx, execResp.ID, client.ExecResizeOptions{
		Height: uint(rows),
		Width:  uint(cols),
	})

	// Tty=true: Docker daemon sends raw PTY bytes (no multiplex framing).
	ds := newDockerStream(resp.HijackedResponse, true)
	ds.execID = execResp.ID
	ds.cli = d.cli
	return ds, nil
}

func (d *DockerRuntime) Attach(ctx context.Context, id string) (Stream, error) {
	info, err := d.cli.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	if err != nil {
		return nil, fmt.Errorf("inspect container: %w", err)
	}
	resp, err := d.cli.ContainerAttach(ctx, id, client.ContainerAttachOptions{
		Stream: true,
		Stdin:  true,
		Stdout: true,
		Stderr: true,
	})
	if err != nil {
		return nil, fmt.Errorf("attach: %w", err)
	}
	// The stream framing follows the container's Tty setting, not the attach
	// request: raw bytes for Tty containers, multiplexed frames otherwise.
	ds := newDockerStream(resp.HijackedResponse, containerTTY(info))
	ds.containerID = id
	ds.cli = d.cli
	return ds, nil
}

func (d *DockerRuntime) Logs(ctx context.Context, id string, opts LogsOptions) (io.ReadCloser, error) {
	// Inspect is always needed to know whether the log stream is multiplexed.
	info, err := d.cli.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	if err != nil {
		return nil, fmt.Errorf("inspect container: %w", err)
	}
	since := opts.Since
	if since == "" {
		// Default: logs from container start
		if info.Container.State != nil {
			since = info.Container.State.StartedAt
		}
	}

	rc, err := d.cli.ContainerLogs(ctx, id, client.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     opts.Follow,
		Since:      since,
		Timestamps: opts.Timestamps,
	})
	if err != nil {
		return nil, fmt.Errorf("logs: %w", err)
	}
	if containerTTY(info) {
		return rc, nil
	}
	pr, pw := io.Pipe()
	go func() {
		_, err := stdcopy.StdCopy(pw, pw, rc)
		_ = pw.CloseWithError(err)
		_ = rc.Close()
	}()
	return pr, nil
}

func containerTTY(info client.ContainerInspectResult) bool {
	return info.Container.Config != nil && info.Container.Config.Tty
}

// dockerStream wraps a Docker HijackedResponse as a Stream.
// When tty=true the daemon sends raw PTY bytes; when false it uses Docker's
// 8-byte multiplexed framing (stdcopy format).  newDockerStream sets up an
// io.Pipe + stdcopy.StdCopy goroutine for the non-TTY case so that Read()
// always returns clean data regardless of the underlying framing.
type dockerStream struct {
	conn        client.HijackedResponse
	execID      string
	containerID string
	cli         *client.Client
	reader      io.Reader // raw conn.Reader (tty) or demuxed pipe (non-tty)
}

// newDockerStream builds a dockerStream.  pass tty=true when the exec/attach
// was created with Tty:true or attaches to a Tty container (raw PTY stream);
// pass tty=false for multiplexed streams (attach to a non-Tty container).
func newDockerStream(conn client.HijackedResponse, tty bool) *dockerStream {
	var r io.Reader
	if tty {
		r = conn.Reader
	} else {
		pr, pw := io.Pipe()
		go func() {
			_, err := stdcopy.StdCopy(pw, pw, conn.Reader)
			_ = pw.CloseWithError(err)
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
		_, err := s.cli.ExecResize(ctx, s.execID, client.ExecResizeOptions{
			Width:  uint(cols),
			Height: uint(rows),
		})
		return err
	}
	if s.containerID != "" {
		_, err := s.cli.ContainerResize(ctx, s.containerID, client.ContainerResizeOptions{
			Width:  uint(cols),
			Height: uint(rows),
		})
		return err
	}
	return nil
}

func (s *dockerStream) Close() error {
	s.conn.Close()
	return nil
}
