package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

// Payload is the JSON body sent to each webhook URL.
type Payload struct {
	Event     string `json:"event"`      // "session.start" or "session.end"
	SessionID string `json:"session_id"`
	Container string `json:"container_id"`
	Mode      string `json:"mode"`
}

// Notifier sends webhook notifications to registered URLs.
type Notifier struct {
	urls   []string
	client *http.Client
}

// New creates a Notifier that posts to the given URLs.
func New(urls []string) *Notifier {
	return &Notifier{
		urls: urls,
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

// Send posts payload to all registered URLs in parallel.
// Failures are logged and ignored so they never block the caller.
func (n *Notifier) Send(payload Payload) {
	if len(n.urls) == 0 {
		return
	}
	body, err := json.Marshal(payload)
	if err != nil {
		slog.Error("webhook marshal", "err", err)
		return
	}
	for _, u := range n.urls {
		go func(url string) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
			if err != nil {
				slog.Warn("webhook request", "url", url, "err", err)
				return
			}
			req.Header.Set("Content-Type", "application/json")
			resp, err := n.client.Do(req)
			if err != nil {
				slog.Warn("webhook send", "url", url, "err", err)
				return
			}
			_ = resp.Body.Close()
			if resp.StatusCode >= 400 {
				slog.Warn("webhook response", "url", url, "status", resp.StatusCode)
			}
		}(u)
	}
}
