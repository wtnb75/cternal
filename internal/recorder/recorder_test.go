package recorder_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wtnb75/cternal/internal/recorder"
)

// ── Recorder ─────────────────────────────────────────────────────────────────

func TestRecorder_empty(t *testing.T) {
	r := recorder.New()
	assert.Equal(t, 0, r.Len())
	assert.Empty(t, r.All())
	assert.Nil(t, r.EventsSince(0))
}

func TestRecorder_addAndRetrieve(t *testing.T) {
	r := recorder.New()
	r.Add(recorder.EventOutput, "hello")
	r.Add(recorder.EventInput, "x")
	assert.Equal(t, 2, r.Len())
	all := r.All()
	assert.Equal(t, recorder.EventOutput, all[0].Type)
	assert.Equal(t, "hello", all[0].Data)
	assert.Equal(t, recorder.EventInput, all[1].Type)
}

func TestRecorder_eventsSince_boundaries(t *testing.T) {
	r := recorder.New()
	r.Add(recorder.EventOutput, "a")
	r.Add(recorder.EventOutput, "b")
	r.Add(recorder.EventOutput, "c")

	assert.Len(t, r.EventsSince(0), 3)
	assert.Len(t, r.EventsSince(1), 2)
	assert.Len(t, r.EventsSince(2), 1)
	assert.Nil(t, r.EventsSince(3))
	assert.Nil(t, r.EventsSince(999))
}

func TestRecorder_concurrent(t *testing.T) {
	r := recorder.New()
	var wg sync.WaitGroup
	const n = 100

	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r.Add(recorder.EventOutput, strings.Repeat("x", i))
		}(i)
	}
	// Concurrent reads while writes happen
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = r.Len()
			_ = r.All()
		}()
	}
	wg.Wait()
	assert.Equal(t, n, r.Len())
}

// ── Player ────────────────────────────────────────────────────────────────────

func TestPlayer_empty(t *testing.T) {
	p := recorder.NewPlayer(nil, 1.0)
	var buf bytes.Buffer
	done := make(chan struct{})
	close(done)
	require.NoError(t, p.Play(&buf, done))
	assert.Empty(t, buf.String())
}

func TestPlayer_outputOnly(t *testing.T) {
	events := []recorder.Event{
		{Time: 0, Type: recorder.EventOutput, Data: "hello"},
		{Time: 0, Type: recorder.EventInput, Data: "x"}, // should not be written
		{Time: 0, Type: recorder.EventOutput, Data: " world"},
	}
	p := recorder.NewPlayer(events, 1.0)
	var buf bytes.Buffer
	done := make(chan struct{})
	close(done) // instant, no delays
	require.NoError(t, p.Play(&buf, done))
	assert.Equal(t, "hello world", buf.String())
}

func TestPlayer_seekFrom(t *testing.T) {
	events := []recorder.Event{
		{Time: 0, Type: recorder.EventOutput, Data: "a"},
		{Time: 0, Type: recorder.EventOutput, Data: "b"},
		{Time: 0, Type: recorder.EventOutput, Data: "c"},
	}
	cases := []struct {
		seek int
		want string
	}{
		{0, "abc"},
		{1, "bc"},
		{2, "c"},
		{3, ""},
		{99, ""},
	}
	for _, tc := range cases {
		p := recorder.NewPlayerFrom(events, 1.0, tc.seek)
		var buf bytes.Buffer
		done := make(chan struct{})
		close(done)
		require.NoError(t, p.Play(&buf, done))
		assert.Equal(t, tc.want, buf.String(), "seek=%d", tc.seek)
	}
}

func TestPlayer_defaultSpeed(t *testing.T) {
	events := []recorder.Event{
		{Time: 0, Type: recorder.EventOutput, Data: "x"},
	}
	// speed=0 should default to 1.0 without panic
	p := recorder.NewPlayer(events, 0)
	var buf bytes.Buffer
	done := make(chan struct{})
	close(done)
	require.NoError(t, p.Play(&buf, done))
	assert.Equal(t, "x", buf.String())
}

// ── Cast (asciicast v3) ───────────────────────────────────────────────────────

func TestMarshal_empty(t *testing.T) {
	hdr := recorder.Header{Width: 80, Height: 24}
	data, err := recorder.Marshal(hdr, nil)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	require.Len(t, lines, 1)

	var got recorder.Header
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &got))
	assert.Equal(t, 3, got.Version)
	assert.Equal(t, 80, got.Width)
	assert.Equal(t, 24, got.Height)
}

func TestMarshal_outputEventsOnly(t *testing.T) {
	hdr := recorder.Header{Width: 80, Height: 24}
	events := []recorder.Event{
		{Time: 100 * time.Millisecond, Type: recorder.EventOutput, Data: "hello"},
		{Time: 200 * time.Millisecond, Type: recorder.EventInput, Data: "x"}, // excluded
		{Time: 300 * time.Millisecond, Type: recorder.EventResize, Data: "80x24"},
		{Time: 400 * time.Millisecond, Type: recorder.EventOutput, Data: "world"},
	}
	data, err := recorder.Marshal(hdr, events)
	require.NoError(t, err)

	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	// header + 2 output events
	require.Len(t, lines, 3)

	var row []any
	require.NoError(t, json.Unmarshal([]byte(lines[1]), &row))
	assert.InDelta(t, 0.1, row[0], 0.001)
	assert.Equal(t, "o", row[1])
	assert.Equal(t, "hello", row[2])
}
