package local

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"testing"
	"time"

	"github.com/freeeve/libcatalog/backend/auth"
	"github.com/freeeve/libcatalog/backend/store"
)

func newService(t *testing.T) (*Service, *store.Mem) {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	db := store.NewMem()
	svc, err := New(db, key, "lcatd-test")
	if err != nil {
		t.Fatal(err)
	}
	return svc, db
}

func TestPasswordHashing(t *testing.T) {
	hash, err := hashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := verifyPassword("correct horse battery staple", hash); err != nil || !ok {
		t.Fatalf("right password: %v, %v", ok, err)
	}
	if ok, _ := verifyPassword("wrong", hash); ok {
		t.Fatal("wrong password accepted")
	}
	hash2, _ := hashPassword("correct horse battery staple")
	if hash == hash2 {
		t.Fatal("salt not randomized")
	}
	if _, err := verifyPassword("x", "$bcrypt$whatever"); err == nil {
		t.Fatal("malformed hash accepted")
	}
}

func TestLoginVerifyRoundTrip(t *testing.T) {
	svc, _ := newService(t)
	if err := svc.CreateUser(t.Context(), "Cat@Example.org", "Cat", "hunter2hunter2", []auth.Role{auth.RoleLibrarian}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	// Email normalization: login with different case.
	tokens, err := svc.Login(t.Context(), "cat@example.org", "hunter2hunter2")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	id, err := svc.Verify(t.Context(), tokens.AccessToken)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if id.Email != "cat@example.org" || !id.CanPublish() || id.CanAdmin() || id.Issuer != "lcatd-test" {
		t.Fatalf("identity = %+v", id)
	}
	if _, err := svc.Login(t.Context(), "cat@example.org", "wrong"); !errors.Is(err, ErrBadCredentials) {
		t.Fatalf("wrong password: %v", err)
	}
	if _, err := svc.Login(t.Context(), "ghost@example.org", "hunter2hunter2"); !errors.Is(err, ErrBadCredentials) {
		t.Fatalf("unknown user: %v", err)
	}
}

func TestAccessTokenExpiry(t *testing.T) {
	svc, _ := newService(t)
	now := time.Now()
	svc.SetClock(func() time.Time { return now })
	_ = svc.CreateUser(t.Context(), "a@example.org", "", "password1", []auth.Role{auth.RoleAdmin})
	tokens, err := svc.Login(t.Context(), "a@example.org", "password1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Verify(t.Context(), tokens.AccessToken); err != nil {
		t.Fatalf("fresh token: %v", err)
	}
	now = now.Add(time.Hour)
	if _, err := svc.Verify(t.Context(), tokens.AccessToken); !errors.Is(err, auth.ErrUnauthorized) {
		t.Fatalf("expired token: %v", err)
	}
}

func TestRefreshRotation(t *testing.T) {
	svc, _ := newService(t)
	_ = svc.CreateUser(t.Context(), "a@example.org", "", "password1", []auth.Role{auth.RoleModerator})
	tokens, err := svc.Login(t.Context(), "a@example.org", "password1")
	if err != nil {
		t.Fatal(err)
	}
	rotated, err := svc.Refresh(t.Context(), tokens.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if rotated.RefreshToken == tokens.RefreshToken {
		t.Fatal("refresh token not rotated")
	}
	// Reuse of the rotated-away token fails (theft detection posture).
	if _, err := svc.Refresh(t.Context(), tokens.RefreshToken); !errors.Is(err, ErrBadCredentials) {
		t.Fatalf("reused refresh: %v", err)
	}
	if _, err := svc.Verify(t.Context(), rotated.AccessToken); err != nil {
		t.Fatalf("rotated access token: %v", err)
	}
	// Logout retires the live token.
	if err := svc.Logout(t.Context(), rotated.RefreshToken); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Refresh(t.Context(), rotated.RefreshToken); !errors.Is(err, ErrBadCredentials) {
		t.Fatalf("refresh after logout: %v", err)
	}
}

func TestLoginRateLimit(t *testing.T) {
	svc, _ := newService(t)
	_ = svc.CreateUser(t.Context(), "a@example.org", "", "password1", []auth.Role{auth.RolePatron})
	for range loginFailureCap {
		if _, err := svc.Login(t.Context(), "a@example.org", "wrong"); !errors.Is(err, ErrBadCredentials) {
			t.Fatalf("failure: %v", err)
		}
	}
	// Even the right password is now rejected for the window.
	if _, err := svc.Login(t.Context(), "a@example.org", "password1"); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("after cap: %v, want ErrRateLimited", err)
	}
}

func TestUserManagement(t *testing.T) {
	svc, _ := newService(t)
	if err := svc.CreateUser(t.Context(), "a@example.org", "A", "password1", []auth.Role{auth.RolePatron}); err != nil {
		t.Fatal(err)
	}
	if err := svc.CreateUser(t.Context(), "a@example.org", "", "password1", nil); !errors.Is(err, ErrUserExists) {
		t.Fatalf("duplicate: %v", err)
	}
	if err := svc.CreateUser(t.Context(), "bad-email", "", "password1", nil); err == nil {
		t.Fatal("bad email accepted")
	}
	if err := svc.CreateUser(t.Context(), "b@example.org", "", "short", nil); err == nil {
		t.Fatal("short password accepted")
	}
	if err := svc.CreateUser(t.Context(), "b@example.org", "", "password1", []auth.Role{"janitor"}); err == nil {
		t.Fatal("unknown role accepted")
	}
	_ = svc.CreateUser(t.Context(), "b@example.org", "B", "password1", []auth.Role{auth.RoleLibrarian})
	if err := svc.SetRoles(t.Context(), "a@example.org", []auth.Role{auth.RoleAdmin}); err != nil {
		t.Fatal(err)
	}
	users, err := svc.ListUsers(t.Context())
	if err != nil || len(users) != 2 {
		t.Fatalf("ListUsers = %v, %v", users, err)
	}
	if users[0].Email != "a@example.org" || users[0].Roles[0] != auth.RoleAdmin {
		t.Fatalf("users[0] = %+v", users[0])
	}
	if err := svc.SetPassword(t.Context(), "b@example.org", "newpassword"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Login(t.Context(), "b@example.org", "newpassword"); err != nil {
		t.Fatalf("login after password change: %v", err)
	}
	if err := svc.DeleteUser(t.Context(), "b@example.org"); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteUser(t.Context(), "b@example.org"); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("double delete: %v", err)
	}
	if _, err := svc.Login(t.Context(), "b@example.org", "newpassword"); !errors.Is(err, ErrBadCredentials) {
		t.Fatalf("login after delete: %v", err)
	}
}

func TestBootstrap(t *testing.T) {
	svc, _ := newService(t)
	if err := svc.Bootstrap(t.Context(), ""); err != nil {
		t.Fatalf("empty spec: %v", err)
	}
	if err := svc.Bootstrap(t.Context(), "not-a-spec"); err == nil {
		t.Fatal("malformed spec accepted")
	}
	if err := svc.Bootstrap(t.Context(), "root@example.org:changeme123"); err != nil {
		t.Fatal(err)
	}
	// Idempotent on reboot.
	if err := svc.Bootstrap(t.Context(), "root@example.org:changeme123"); err != nil {
		t.Fatalf("second bootstrap: %v", err)
	}
	tokens, err := svc.Login(t.Context(), "root@example.org", "changeme123")
	if err != nil {
		t.Fatal(err)
	}
	id, err := svc.Verify(t.Context(), tokens.AccessToken)
	if err != nil || !id.CanAdmin() {
		t.Fatalf("bootstrap admin: %+v, %v", id, err)
	}
}
