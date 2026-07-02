package trigger

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestWebhookSignedDelivery(t *testing.T) {
	secret := []byte("0123456789abcdef")
	var gotBody []byte
	var gotSig string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		gotSig = r.Header.Get(SignatureHeader)
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)

	hook := Webhook{URL: srv.URL, Secret: secret, Client: srv.Client()}
	event := Event{Kind: "grains-changed", Paths: []string{"data/works/aa/w1.nq"}, At: time.Now().UTC()}
	if err := hook.Notify(t.Context(), event); err != nil {
		t.Fatalf("Notify: %v", err)
	}
	if !Verify(secret, gotBody, gotSig) {
		t.Fatal("signature does not verify")
	}
	if Verify([]byte("wrong-secret-00"), gotBody, gotSig) {
		t.Fatal("wrong secret verified")
	}
	var decoded Event
	if err := json.Unmarshal(gotBody, &decoded); err != nil || decoded.Kind != "grains-changed" || len(decoded.Paths) != 1 {
		t.Fatalf("body = %s (%v)", gotBody, err)
	}
}

func TestWebhookErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusBadGateway)
	}))
	t.Cleanup(srv.Close)
	hook := Webhook{URL: srv.URL, Secret: []byte("0123456789abcdef"), Client: srv.Client()}
	if err := hook.Notify(t.Context(), Event{Kind: "x"}); err == nil {
		t.Fatal("non-2xx accepted")
	}
}

func TestNoop(t *testing.T) {
	if err := (Noop{}).Notify(t.Context(), Event{}); err != nil {
		t.Fatal(err)
	}
}
