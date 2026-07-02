// Package httpapi assembles the backend's HTTP surface as a plain
// net/http.Handler, independent of how it is served: cmd/lcatd wraps it in a
// listener, cmd/lcatd-lambda wraps it in the Lambda runtime. Handlers arrive
// in later tasks; this package owns routing, middleware, and response
// conventions.
package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/freeeve/libcatalog/backend/auth"
	"github.com/freeeve/libcatalog/storage/blob"
)

// Deps carries the services handlers depend on. It grows as tasks land;
// everything in it is an interface so tests inject fakes.
type Deps struct {
	// Logger receives request logs and handler errors. nil disables logging.
	Logger *slog.Logger
	// Blob is the grain store. Record and export handlers (later tasks)
	// read and publish through it.
	Blob blob.Store
	// Verifier authenticates staff bearer tokens (an auth.Multi when both
	// SSO and local users are configured). nil leaves staff routes
	// unregistered.
	Verifier auth.TokenVerifier
	// AuthExchange, when set, serves POST /v1/auth/exchange -- the OIDC
	// PKCE token-exchange proxy for SPA logins against an external issuer.
	AuthExchange http.Handler
}

// New assembles the routed, middleware-wrapped API handler.
func New(deps Deps) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/healthz", handleHealthz)
	if deps.AuthExchange != nil {
		mux.Handle("POST /v1/auth/exchange", deps.AuthExchange)
	}
	return wrap(mux, deps.Logger)
}

func handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
