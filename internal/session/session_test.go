package session_test

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wtnb75/cternal/internal/session"
)

// ── Store ─────────────────────────────────────────────────────────────────────

func newSession(id string) *session.Session {
	return session.NewSession(id, "ctr-"+id, session.ModeExec, nil)
}

func TestStore_createAndGet(t *testing.T) {
	store := session.NewStore(10)
	sess := newSession("s1")
	require.NoError(t, store.Create(sess))
	got, err := store.Get("s1")
	require.NoError(t, err)
	assert.Equal(t, sess, got)
}

func TestStore_getNotFound(t *testing.T) {
	store := session.NewStore(10)
	_, err := store.Get("missing")
	assert.ErrorIs(t, err, session.ErrSessionNotFound)
}

func TestStore_maxSessions(t *testing.T) {
	store := session.NewStore(2)
	require.NoError(t, store.Create(newSession("s1")))
	require.NoError(t, store.Create(newSession("s2")))
	err := store.Create(newSession("s3"))
	assert.ErrorIs(t, err, session.ErrMaxSessions)
	assert.Equal(t, 2, store.Len())
}

func TestStore_delete(t *testing.T) {
	store := session.NewStore(10)
	require.NoError(t, store.Create(newSession("s1")))
	store.Delete("s1")
	_, err := store.Get("s1")
	assert.ErrorIs(t, err, session.ErrSessionNotFound)
}

func TestStore_list(t *testing.T) {
	store := session.NewStore(10)
	require.NoError(t, store.Create(newSession("s1")))
	require.NoError(t, store.Create(newSession("s2")))
	list := store.List()
	assert.Len(t, list, 2)
}

func TestStore_concurrent_create(t *testing.T) {
	store := session.NewStore(0) // unlimited
	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := string(rune('a' + i%26))
			_ = store.Create(newSession(id))
		}(i)
	}
	wg.Wait()
}

func TestStore_concurrent_maxSessions(t *testing.T) {
	const max = 5
	store := session.NewStore(max)
	var wg sync.WaitGroup
	var created, rejected int
	var mu sync.Mutex

	for i := range max * 3 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := string([]byte{byte('A') + byte(i%52), byte('0' + i%10)})
			err := store.Create(newSession(id))
			mu.Lock()
			if err == nil {
				created++
			} else {
				rejected++
			}
			mu.Unlock()
		}(i)
	}
	wg.Wait()
	assert.LessOrEqual(t, created, max)
	assert.Greater(t, rejected, 0)
}

func TestStore_getByContainer(t *testing.T) {
	store := session.NewStore(10)
	sess := newSession("s1")
	require.NoError(t, store.Create(sess))

	got, err := store.GetByContainer("ctr-s1", session.ModeExec)
	require.NoError(t, err)
	assert.Equal(t, sess, got)

	_, err = store.GetByContainer("ctr-s1", session.ModeAttach)
	assert.ErrorIs(t, err, session.ErrSessionNotFound)
}

// ── Session subscribe/unsubscribe ──────────────────────────────────────────────

func TestSession_subscribeUnsubscribe(t *testing.T) {
	sess := newSession("s1")
	sub := sess.Subscribe()
	assert.Equal(t, 1, sess.SubscriberCount())

	sess.Broadcast([]byte("hello"))
	select {
	case data := <-sub.Ch:
		assert.Equal(t, []byte("hello"), data)
	case <-time.After(time.Second):
		t.Fatal("broadcast not received")
	}

	sess.Unsubscribe(sub)
	assert.Equal(t, 0, sess.SubscriberCount())
	// Done channel should be closed
	select {
	case <-sub.Done:
	default:
		t.Fatal("Done not closed after Unsubscribe")
	}
}

func TestSession_broadcast_drop(t *testing.T) {
	sess := newSession("s1")
	sub := sess.Subscribe()
	// Fill the buffer
	for range 64 {
		sess.Broadcast([]byte("x"))
	}
	// One more should be dropped without blocking
	sess.Broadcast([]byte("overflow"))
	_ = sub
}

func TestSession_concurrent_subscribeUnsubscribe(t *testing.T) {
	sess := newSession("s1")
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sub := sess.Subscribe()
			sess.Broadcast([]byte("data"))
			sess.Unsubscribe(sub)
		}()
	}
	wg.Wait()
	assert.Equal(t, 0, sess.SubscriberCount())
}

// ── TTLManager ────────────────────────────────────────────────────────────────

func TestTTL_evicts(t *testing.T) {
	evicted := make(chan string, 1)
	mgr := session.NewTTLManager(50*time.Millisecond, func(id string) {
		evicted <- id
	})
	mgr.StartTTL("s1")
	select {
	case id := <-evicted:
		assert.Equal(t, "s1", id)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("TTL did not fire")
	}
}

func TestTTL_cancelPreventsEviction(t *testing.T) {
	evicted := make(chan string, 1)
	mgr := session.NewTTLManager(100*time.Millisecond, func(id string) {
		evicted <- id
	})
	mgr.StartTTL("s1")
	stopped := mgr.CancelTTL("s1")
	assert.True(t, stopped)

	select {
	case <-evicted:
		t.Fatal("should not have been evicted")
	case <-time.After(200 * time.Millisecond):
		// correct: no eviction
	}
}

func TestTTL_reconnectRace(t *testing.T) {
	// Simulate: TTL fires just as a client reconnects.
	// CancelTTL may return false if the timer already fired;
	// the session should still survive because the reconnect wins.
	evicted := make(chan string, 1)
	mgr := session.NewTTLManager(1*time.Millisecond, func(id string) {
		evicted <- id
	})
	mgr.StartTTL("s1")
	// Don't assert on Cancel result — the race is intentional.
	mgr.CancelTTL("s1")
	// If eviction happened, it's fine — the test just validates no deadlock/panic.
}

func TestTTL_remove(t *testing.T) {
	evicted := make(chan string, 1)
	mgr := session.NewTTLManager(100*time.Millisecond, func(id string) {
		evicted <- id
	})
	mgr.StartTTL("s1")
	mgr.Remove("s1")
	select {
	case <-evicted:
		t.Fatal("should not have been evicted after Remove")
	case <-time.After(200 * time.Millisecond):
		// correct
	}
}
