package auth

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRoleCapabilities(t *testing.T) {
	cases := []struct {
		roles                        []Role
		moderate, publish, adminable bool
	}{
		{nil, false, false, false},
		{[]Role{RolePatron}, false, false, false},
		{[]Role{RoleModerator}, true, false, false},
		{[]Role{RoleLibrarian}, true, true, false},
		{[]Role{RoleAdmin}, true, true, true},
		{[]Role{RolePatron, RoleLibrarian}, true, true, false},
	}
	for _, tc := range cases {
		id := Identity{Roles: tc.roles}
		if id.CanModerate() != tc.moderate || id.CanPublish() != tc.publish || id.CanAdmin() != tc.adminable {
			t.Errorf("roles %v: moderate/publish/admin = %v/%v/%v, want %v/%v/%v",
				tc.roles, id.CanModerate(), id.CanPublish(), id.CanAdmin(), tc.moderate, tc.publish, tc.adminable)
		}
	}
	if !Role("librarian").Valid() || Role("nope").Valid() {
		t.Fatal("Role.Valid misclassifies")
	}
}

// fakeVerifier accepts one exact token string.
type fakeVerifier struct {
	token string
	id    Identity
	err   error
}

func (f fakeVerifier) Verify(ctx context.Context, raw string) (Identity, error) {
	if f.err != nil {
		return Identity{}, f.err
	}
	if raw != f.token {
		return Identity{}, ErrUnauthorized
	}
	return f.id, nil
}

// jwtWith builds an unsigned JWT-shaped token with the given payload JSON --
// enough for Multi's unverified issuer routing.
func jwtWith(payload string) string {
	enc := base64.RawURLEncoding.EncodeToString
	return enc([]byte(`{"alg":"none"}`)) + "." + enc([]byte(payload)) + "." + enc([]byte("sig"))
}

func TestMultiDispatch(t *testing.T) {
	tokA := jwtWith(`{"iss":"https://a.example"}`)
	tokB := jwtWith(`{"iss":"https://b.example"}`)
	m := NewMulti(map[string]TokenVerifier{
		"https://a.example": fakeVerifier{token: tokA, id: Identity{Subject: "ua", Issuer: "https://a.example"}},
		"https://b.example": fakeVerifier{token: tokB, id: Identity{Subject: "ub", Issuer: "https://b.example"}},
	})
	id, err := m.Verify(t.Context(), tokA)
	if err != nil || id.Subject != "ua" {
		t.Fatalf("issuer a: %+v, %v", id, err)
	}
	id, err = m.Verify(t.Context(), tokB)
	if err != nil || id.Subject != "ub" {
		t.Fatalf("issuer b: %+v, %v", id, err)
	}
	if _, err := m.Verify(t.Context(), jwtWith(`{"iss":"https://unknown.example"}`)); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("unknown issuer: err = %v", err)
	}
	for _, bad := range []string{"", "notajwt", "a.b", jwtWith(`{}`), "x.!!!.z"} {
		if _, err := m.Verify(t.Context(), bad); !errors.Is(err, ErrUnauthorized) {
			t.Fatalf("malformed token %q: err = %v", bad, err)
		}
	}
}

func TestRequireMiddleware(t *testing.T) {
	verifier := fakeVerifier{
		token: "good",
		id:    Identity{Subject: "u1", Email: "cat@example.org", Roles: []Role{RoleModerator}},
	}
	var seen Identity
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen, _ = FromContext(r.Context())
		w.WriteHeader(http.StatusNoContent)
	})

	call := func(header string, role Role) int {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/v1/queue", nil)
		if header != "" {
			req.Header.Set("Authorization", header)
		}
		Require(verifier, role)(inner).ServeHTTP(rec, req)
		return rec.Code
	}

	if code := call("", RoleModerator); code != http.StatusUnauthorized {
		t.Fatalf("no token: %d, want 401", code)
	}
	if code := call("Bearer wrong", RoleModerator); code != http.StatusUnauthorized {
		t.Fatalf("bad token: %d, want 401", code)
	}
	if code := call("Bearer good", RoleLibrarian); code != http.StatusForbidden {
		t.Fatalf("insufficient role: %d, want 403", code)
	}
	if code := call("Bearer good", RoleModerator); code != http.StatusNoContent {
		t.Fatalf("sufficient role: %d, want 204", code)
	}
	if seen.Subject != "u1" || seen.Email != "cat@example.org" {
		t.Fatalf("identity in context = %+v", seen)
	}
	forbidden := fakeVerifier{err: ErrForbidden}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer x")
	Require(forbidden, RolePatron)(inner).ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("verifier ErrForbidden: %d, want 403", rec.Code)
	}
}
