package webhook_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wtnb75/cternal/internal/webhook"
)

func TestNotifier_send(t *testing.T) {
	received := make(chan webhook.Payload, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p webhook.Payload
		require.NoError(t, json.NewDecoder(r.Body).Decode(&p))
		received <- p
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	n := webhook.New([]string{srv.URL})
	n.Send(webhook.Payload{
		Event:     "session.start",
		SessionID: "s1",
		Container: "ctr1",
		Mode:      "exec",
	})

	select {
	case p := <-received:
		assert.Equal(t, "session.start", p.Event)
		assert.Equal(t, "s1", p.SessionID)
	case <-time.After(3 * time.Second):
		t.Fatal("webhook not received")
	}
}

func TestNotifier_noURLs(t *testing.T) {
	n := webhook.New(nil)
	// Should not panic or block
	n.Send(webhook.Payload{Event: "session.start"})
}

func TestNotifier_failureIgnored(t *testing.T) {
	// Server that returns 500 — should not cause the caller to fail
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	n := webhook.New([]string{srv.URL})
	n.Send(webhook.Payload{Event: "session.end"})
	// Give goroutine time to complete without blocking
	time.Sleep(100 * time.Millisecond)
}

func TestNotifier_parallelDelivery(t *testing.T) {
	const count = 3
	received := make(chan struct{}, count)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received <- struct{}{}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	urls := make([]string, count)
	for i := range urls {
		urls[i] = srv.URL
	}
	n := webhook.New(urls)
	n.Send(webhook.Payload{Event: "test"})

	deadline := time.After(3 * time.Second)
	got := 0
	for got < count {
		select {
		case <-received:
			got++
		case <-deadline:
			t.Fatalf("only received %d/%d webhook deliveries", got, count)
		}
	}
}
