// Package trigger notifies downstream rebuild machinery that grains changed:
// after a publish, something must re-run serialize/project/index and redeploy
// the static site. The Notifier seam keeps that transport pluggable -- a
// webhook toward any CI system here, SQS/EventBridge in trigger/awstrigger --
// and noop for setups that rebuild on a schedule.
package trigger

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Event describes one batch of grain changes.
type Event struct {
	// Kind is the event class, e.g. "grains-changed".
	Kind string `json:"kind"`
	// Paths lists the changed blob paths (grain files).
	Paths []string  `json:"paths"`
	At    time.Time `json:"at"`
}

// Notifier delivers events to the rebuild pipeline.
type Notifier interface {
	Notify(ctx context.Context, e Event) error
}

// Noop discards events (scheduled-rebuild deployments).
type Noop struct{}

// Notify implements Notifier.
func (Noop) Notify(context.Context, Event) error { return nil }

// SignatureHeader carries the webhook body's HMAC-SHA256 (hex).
const SignatureHeader = "X-Lcat-Signature"

// Webhook POSTs each event as JSON to a URL, signed with an HMAC so the
// receiver can authenticate the sender (CI webhook endpoints are otherwise
// unauthenticated).
type Webhook struct {
	URL    string
	Secret []byte
	// Client overrides the HTTP client (tests). nil = http.DefaultClient.
	Client *http.Client
}

// Notify implements Notifier.
func (w Webhook) Notify(ctx context.Context, e Event) error {
	body, err := json.Marshal(e)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(SignatureHeader, Sign(w.Secret, body))
	client := w.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("trigger: webhook status %d", resp.StatusCode)
	}
	return nil
}

// Sign returns the hex HMAC-SHA256 of body under secret.
func Sign(secret, body []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// Verify checks a webhook body against its signature header value.
func Verify(secret, body []byte, signature string) bool {
	return hmac.Equal([]byte(Sign(secret, body)), []byte(signature))
}
