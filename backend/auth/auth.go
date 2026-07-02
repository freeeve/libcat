// Package auth defines the backend's identity model -- roles with capability
// checks, the TokenVerifier seam, multi-issuer dispatch, and the HTTP
// middleware guarding staff routes. Concrete verifiers live in subpackages:
// auth/oidc (external SSO issuers) and auth/local (built-in users); either or
// both may be configured, so a deployment can start with local users and add
// an IdP later without touching handlers.
package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

// Role is a caller's privilege tier. Roles are ranked; holding a role grants
// every capability of the ranks below it.
type Role string

const (
	// RolePatron is an authenticated end user with no staff capability.
	RolePatron Role = "patron"
	// RoleModerator triages the review queue but cannot publish to the
	// graph, add manual terms, or read the audit log.
	RoleModerator Role = "moderator"
	// RoleLibrarian catalogs: publishes, edits records, runs exports.
	RoleLibrarian Role = "librarian"
	// RoleAdmin additionally manages users, configuration, and enrichment.
	RoleAdmin Role = "admin"
)

var roleRank = map[Role]int{RolePatron: 1, RoleModerator: 2, RoleLibrarian: 3, RoleAdmin: 4}

// Valid reports whether r is a known role.
func (r Role) Valid() bool { return roleRank[r] != 0 }

// Sentinel errors mapped to HTTP statuses by the middleware.
var (
	ErrUnauthorized = errors.New("auth: missing or invalid token") // 401
	ErrForbidden    = errors.New("auth: insufficient role")        // 403
)

// Identity is a verified caller.
type Identity struct {
	// Subject is the issuer-scoped stable id (OIDC sub / local user id).
	Subject string
	Email   string
	Name    string
	Roles   []Role
	// Issuer identifies which configured verifier accepted the token.
	Issuer string
}

func (id Identity) rank() int {
	max := 0
	for _, r := range id.Roles {
		if roleRank[r] > max {
			max = roleRank[r]
		}
	}
	return max
}

// Has reports whether the identity holds role or a higher-ranked one.
func (id Identity) Has(role Role) bool { return id.rank() >= roleRank[role] }

// CanModerate reports queue-triage capability.
func (id Identity) CanModerate() bool { return id.Has(RoleModerator) }

// CanPublish reports graph-publishing (cataloger) capability.
func (id Identity) CanPublish() bool { return id.Has(RoleLibrarian) }

// CanAdmin reports administrative capability.
func (id Identity) CanAdmin() bool { return id.Has(RoleAdmin) }

// TokenVerifier validates a raw bearer token and returns the caller.
// Implementations return ErrUnauthorized (possibly wrapped) for tokens they
// reject.
type TokenVerifier interface {
	Verify(ctx context.Context, raw string) (Identity, error)
}

// Multi dispatches verification by the token's (unverified) issuer claim to
// the verifier configured for that issuer, which then performs the real
// signature and claim checks. Unknown issuers are rejected.
type Multi struct {
	verifiers map[string]TokenVerifier
}

// NewMulti maps issuer URLs to their verifiers.
func NewMulti(verifiers map[string]TokenVerifier) *Multi {
	return &Multi{verifiers: verifiers}
}

// Verify routes the token to its issuer's verifier.
func (m *Multi) Verify(ctx context.Context, raw string) (Identity, error) {
	iss, err := unverifiedIssuer(raw)
	if err != nil {
		return Identity{}, ErrUnauthorized
	}
	v, ok := m.verifiers[iss]
	if !ok {
		return Identity{}, ErrUnauthorized
	}
	return v.Verify(ctx, raw)
}

// unverifiedIssuer decodes the JWT payload without verification, solely to
// select which verifier should (dis)prove the token.
func unverifiedIssuer(raw string) (string, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return "", errors.New("auth: not a JWT")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", err
	}
	var claims struct {
		Iss string `json:"iss"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", err
	}
	if claims.Iss == "" {
		return "", errors.New("auth: no issuer claim")
	}
	return claims.Iss, nil
}

type contextKey struct{}

// FromContext returns the verified identity a Require middleware stored.
func FromContext(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(contextKey{}).(Identity)
	return id, ok
}

// WithIdentity returns ctx carrying id; tests and middleware use it.
func WithIdentity(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, contextKey{}, id)
}

// Require wraps next so it only runs for callers whose bearer token verifies
// and whose roles include role (or higher). The identity lands in the request
// context for handlers.
func Require(v TokenVerifier, role Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			if !ok || raw == "" {
				writeAuthError(w, http.StatusUnauthorized, "missing bearer token")
				return
			}
			id, err := v.Verify(r.Context(), raw)
			if err != nil {
				if errors.Is(err, ErrForbidden) {
					writeAuthError(w, http.StatusForbidden, "insufficient role")
					return
				}
				writeAuthError(w, http.StatusUnauthorized, "invalid token")
				return
			}
			if !id.Has(role) {
				writeAuthError(w, http.StatusForbidden, "insufficient role")
				return
			}
			next.ServeHTTP(w, r.WithContext(WithIdentity(r.Context(), id)))
		})
	}
}

func writeAuthError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
