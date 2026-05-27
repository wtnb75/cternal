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

func TestRecorder_addAt_storesExplicitElapsed(t *testing.T) {
	r := recorder.New()
	r.AddAt(recorder.EventOutput, "a", 0)
	r.AddAt(recorder.EventOutput, "b", 5*time.Second)
	r.AddAt(recorder.EventOutput, "c", 5*time.Second+200*time.Millisecond)

	all := r.All()
	require.Len(t, all, 3)
	assert.Equal(t, time.Duration(0), all[0].Time)
	assert.Equal(t, 5*time.Second, all[1].Time)
	assert.Equal(t, 5*time.Second+200*time.Millisecond, all[2].Time)
}

func TestRecorder_addAt_mixedWithAdd(t *testing.T) {
	// AddAt and Add can be interleaved; each appends in call order.
	r := recorder.New()
	r.AddAt(recorder.EventOutput, "x", 10*time.Second)
	r.Add(recorder.EventOutput, "y") // uses wall clock
	require.Equal(t, 2, r.Len())
	all := r.All()
	assert.Equal(t, 10*time.Second, all[0].Time)
	assert.GreaterOrEqual(t, all[1].Time, time.Duration(0)) // wall-clock elapsed ≥ 0
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
	assert.Equal(t, 2, got.Version)
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

func TestMarshal_idleTimeLimit(t *testing.T) {
	// Events: t=0s, t=5s (5s gap), t=5.2s (0.2s gap).
	// With a 1s limit the 5s gap is clamped to 1s, so the adjusted times are:
	//   event0 → 0s   (initial gap 0 ≤ 1s, no clamp)
	//   event1 → 1s   (5s gap clamped to 1s)
	//   event2 → 1.2s (0.2s gap unchanged)
	hdr := recorder.Header{Width: 80, Height: 24, IdleTimeLimit: 1.0}
	events := []recorder.Event{
		{Time: 0, Type: recorder.EventOutput, Data: "a"},
		{Time: 5 * time.Second, Type: recorder.EventOutput, Data: "b"},
		{Time: 5*time.Second + 200*time.Millisecond, Type: recorder.EventOutput, Data: "c"},
	}
	data, err := recorder.Marshal(hdr, events)
	require.NoError(t, err)

	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	require.Len(t, lines, 4) // header + 3 events

	parse := func(line string) float64 {
		var row []any
		require.NoError(t, json.Unmarshal([]byte(line), &row))
		return row[0].(float64)
	}

	assert.InDelta(t, 0.0, parse(lines[1]), 0.001)
	assert.InDelta(t, 1.0, parse(lines[2]), 0.001) // clamped from 5s
	assert.InDelta(t, 1.2, parse(lines[3]), 0.001) // 0.2s gap unchanged
}

func TestMarshal_idleTimeLimit_initialGapClamped(t *testing.T) {
	// First event is 10s after session start; with limit=1s it becomes t=1s.
	hdr := recorder.Header{Width: 80, Height: 24, IdleTimeLimit: 1.0}
	events := []recorder.Event{
		{Time: 10 * time.Second, Type: recorder.EventOutput, Data: "late"},
	}
	data, err := recorder.Marshal(hdr, events)
	require.NoError(t, err)

	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	require.Len(t, lines, 2)

	var row []any
	require.NoError(t, json.Unmarshal([]byte(lines[1]), &row))
	assert.InDelta(t, 1.0, row[0].(float64), 0.001)
}

func TestMarshal_noIdleTimeLimit_preservesGaps(t *testing.T) {
	// Without IdleTimeLimit the original timestamps are preserved.
	hdr := recorder.Header{Width: 80, Height: 24}
	events := []recorder.Event{
		{Time: 0, Type: recorder.EventOutput, Data: "a"},
		{Time: 5 * time.Second, Type: recorder.EventOutput, Data: "b"},
	}
	data, err := recorder.Marshal(hdr, events)
	require.NoError(t, err)

	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	require.Len(t, lines, 3)

	var row []any
	require.NoError(t, json.Unmarshal([]byte(lines[2]), &row))
	assert.InDelta(t, 5.0, row[0].(float64), 0.001)
}
