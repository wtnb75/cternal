package runtime

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/moby/moby/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// muxFrame encodes data as a single Docker multiplexed stream frame.
func muxFrame(stream byte, data string) []byte {
	hdr := make([]byte, 8)
	hdr[0] = stream
	binary.BigEndian.PutUint32(hdr[4:], uint32(len(data)))
	return append(hdr, data...)
}

type fakeDocker struct {
	tty        bool
	inspectErr bool
	payload    []byte // body for logs and attach
}

func (f *fakeDocker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Api-Version", "1.47")
	switch {
	case strings.HasSuffix(r.URL.Path, "/_ping"):
		_, _ = io.WriteString(w, "OK")
	case strings.HasSuffix(r.URL.Path, "/json"):
		if f.inspectErr {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"message": "No such container: x"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"Id":     "x",
			"State":  map[string]any{"StartedAt": "2026-01-01T00:00:00Z"},
			"Config": map[string]any{"Tty": f.tty},
		})
	case strings.HasSuffix(r.URL.Path, "/logs"):
		_, _ = w.Write(f.payload)
	case strings.HasSuffix(r.URL.Path, "/attach"):
		conn, buf, err := w.(http.Hijacker).Hijack()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		ct := "application/vnd.docker.multiplexed-stream"
		if f.tty {
			ct = "application/vnd.docker.raw-stream"
		}
		_, _ = buf.WriteString("HTTP/1.1 101 UPGRADED\r\nContent-Type: " + ct +
			"\r\nConnection: Upgrade\r\nUpgrade: tcp\r\n\r\n")
		_, _ = buf.Write(f.payload)
		_ = buf.Flush()
		// Keep the connection open briefly so the client reads the payload
		// before seeing EOF.
		_, _ = bufio.NewReader(conn).ReadByte()
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func newFakeDockerRuntime(t *testing.T, f *fakeDocker) *DockerRuntime {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	cli, err := client.New(client.WithHost("tcp://"+strings.TrimPrefix(srv.URL, "http://")),
		client.WithAPIVersion("1.47"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = cli.Close() })
	return &DockerRuntime{cli: cli}
}

func readAllStream(t *testing.T, s Stream) (string, error) {
	t.Helper()
	var sb strings.Builder
	deadline := time.After(3 * time.Second)
	type chunk struct {
		b   []byte
		err error
	}
	ch := make(chan chunk)
	go func() {
		for {
			b, err := s.Read()
			ch <- chunk{b, err}
			if err != nil {
				return
			}
		}
	}()
	for {
		select {
		case c := <-ch:
			sb.Write(c.b)
			if c.err != nil {
				return sb.String(), c.err
			}
		case <-deadline:
			t.Fatal("timed out reading stream")
		}
	}
}

func TestDockerRuntime_Logs_streamFormat(t *testing.T) {
	tests := []struct {
		name    string
		tty     bool
		since   string
		payload []byte
		want    string
	}{
		{
			name:    "tty container returns raw bytes unchanged",
			tty:     true,
			payload: []byte("hello-from-logs\r\n"),
			want:    "hello-from-logs\r\n",
		},
		{
			name:    "tty container with explicit since still detects tty",
			tty:     true,
			since:   "2026-01-01T00:00:00Z",
			payload: []byte("raw\r\n"),
			want:    "raw\r\n",
		},
		{
			name:    "non-tty container demuxes stdout and stderr",
			tty:     false,
			payload: append(muxFrame(1, "out\n"), muxFrame(2, "err\n")...),
			want:    "out\nerr\n",
		},
		{
			name:    "tty container with empty output",
			tty:     true,
			payload: nil,
			want:    "",
		},
		{
			name:    "non-tty container with empty output",
			tty:     false,
			payload: nil,
			want:    "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newFakeDockerRuntime(t, &fakeDocker{tty: tt.tty, payload: tt.payload})
			rc, err := d.Logs(context.Background(), "x", LogsOptions{Since: tt.since})
			require.NoError(t, err)
			defer func() { _ = rc.Close() }()
			got, err := io.ReadAll(rc)
			require.NoError(t, err)
			assert.Equal(t, tt.want, string(got))
		})
	}
}

func TestDockerRuntime_Logs_malformedMuxReturnsError(t *testing.T) {
	// A non-tty container must use multiplexed framing; anything else should
	// surface as an error instead of a silent empty stream.
	d := newFakeDockerRuntime(t, &fakeDocker{tty: false, payload: []byte("not-a-mux-frame")})
	rc, err := d.Logs(context.Background(), "x", LogsOptions{})
	require.NoError(t, err)
	defer func() { _ = rc.Close() }()
	_, err = io.ReadAll(rc)
	assert.Error(t, err)
}

func TestDockerRuntime_Logs_inspectError(t *testing.T) {
	for _, since := range []string{"", "2026-01-01T00:00:00Z"} {
		t.Run("since="+since, func(t *testing.T) {
			d := newFakeDockerRuntime(t, &fakeDocker{inspectErr: true})
			_, err := d.Logs(context.Background(), "x", LogsOptions{Since: since})
			assert.ErrorContains(t, err, "inspect container")
		})
	}
}

func TestDockerRuntime_Attach_streamFormat(t *testing.T) {
	tests := []struct {
		name    string
		tty     bool
		payload []byte
		want    string
	}{
		{
			name:    "tty container returns raw bytes unchanged",
			tty:     true,
			payload: []byte("ATTACH_OK\r\n"),
			want:    "ATTACH_OK\r\n",
		},
		{
			name:    "non-tty container demuxes stdout and stderr",
			tty:     false,
			payload: append(muxFrame(1, "out\n"), muxFrame(2, "err\n")...),
			want:    "out\nerr\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newFakeDockerRuntime(t, &fakeDocker{tty: tt.tty, payload: tt.payload})
			s, err := d.Attach(context.Background(), "x")
			require.NoError(t, err)
			defer func() { _ = s.Close() }()

			var got strings.Builder
			for got.Len() < len(tt.want) {
				b, err := s.Read()
				require.NoError(t, err)
				got.Write(b)
			}
			assert.Equal(t, tt.want, got.String())
		})
	}
}

func TestDockerRuntime_Attach_inspectError(t *testing.T) {
	d := newFakeDockerRuntime(t, &fakeDocker{inspectErr: true})
	_, err := d.Attach(context.Background(), "x")
	assert.ErrorContains(t, err, "inspect container")
}

func TestDockerStream_nonTTY_malformedReturnsError(t *testing.T) {
	d := newFakeDockerRuntime(t, &fakeDocker{tty: false, payload: []byte("not-a-mux-frame")})
	s, err := d.Attach(context.Background(), "x")
	require.NoError(t, err)
	defer func() { _ = s.Close() }()
	_, err = readAllStream(t, s)
	require.Error(t, err)
	assert.False(t, errors.Is(err, io.EOF), "malformed stream must not look like a clean EOF")
}
