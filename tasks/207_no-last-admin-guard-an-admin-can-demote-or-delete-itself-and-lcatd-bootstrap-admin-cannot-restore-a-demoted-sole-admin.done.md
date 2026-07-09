# 207 -- no last-admin guard: an admin can demote or delete itself, and LCATD_BOOTSTRAP_ADMIN cannot restore a demoted sole admin

Filed from libcat-e2e on 2026-07-09 (cross-repo ask).

## Symptom

`PUT /v1/users/{email}/roles` and `DELETE /v1/users/{email}` accept the caller's
own email, with no check that another admin would remain. On this playground
`GET /v1/users` returns exactly **one** user -- `eve@example.org`, roles
`["admin"]` -- so a single request permanently ends all user administration:

```
PUT /v1/users/eve@example.org/roles  {"roles":["librarian"]}   -> 204
```

Nothing in the codebase can undo that. It is not merely "the admin loses admin";
there is no remaining principal anywhere that can grant the role back.

Measured against a **sentinel admin** (`node harness/probe_users.mjs`,
2026-07-09) -- the real admin was never demoted or deleted:

| # | Check | Result |
|---|---|---|
| U7 | sentinel admin can administer | `GET /v1/users` -> 200, `accessTTL=900s` |
| U8 | **admin demotes ITSELF** | `PUT own roles ["librarian"]` -> **204** |
| U9 | **demotion is irreversible for that user** | after re-login: `GET /v1/users` -> 403, `PUT own roles ["admin"]` -> 403 |
| U11 | **admin deletes ITSELF** | `DELETE own account` -> **204** |
| U14 | **bootstrap cannot re-grant admin** | `CreateUser(existing, roles:[admin])` -> 409, roles still `["librarian"]` |
| U12 | deleted user cannot log in or refresh | 401 / 401 (correct) |
| U10 | stale access token keeps admin for <=15 min | 200 (stateless JWT; `Refresh` re-reads roles) -- by design |

## Root cause

No guard exists on either mutation:

- `backend/auth/local/users.go:103` -- `SetRoles` validates that each role name
  is known, then writes. It never inspects who the caller is, nor whether the
  target is the last `RoleAdmin`.
- `backend/auth/local/users.go:151` -- `DeleteUser` deletes unconditionally.
- `backend/httpapi/auth_handlers.go:89` and `:107` -- the handlers pass
  `r.PathValue("email")` straight through. `auth.FromContext(r.Context())` is
  available (the profiles handler uses it at `profiles_handlers.go:71`) but is
  never consulted here, so caller == target is not even detectable downstream.

And the documented recovery hatch does not recover:

- `backend/auth/local/users.go:183-196` -- `Bootstrap` calls
  `CreateUser(email, "", password, []auth.Role{auth.RoleAdmin})` and, on
  `ErrUserExists`, returns `nil`. The comment says "It is a no-op when the user
  already exists, so it is safe to run on every boot" -- it is safe, and also
  inert. A demoted admin still exists, so `LCATD_BOOTSTRAP_ADMIN` re-grants
  nothing.
- `backend/appdeps/appdeps.go:278` is the only caller.

U14 exercises exactly that code path through the API and confirms it: 409, roles
unchanged.

So the two failure modes differ in severity:

- **Self-delete** is recoverable -- the user is gone, so the next boot's
  `Bootstrap` re-creates them as admin.
- **Self-demotion of the sole admin is not recoverable by any supported path.**
  Only hand-editing the user record in the store restores it.

## Why it matters

There is **no Users screen in the SPA** (`backend/ui/src/screens/` has no
`Users.svelte`); user administration is API-only, done by hand with curl. That is
precisely the context where a wrong `{email}` in a URL, or a `roles` array that
was meant for a colleague, is easy to send. The blast radius is the whole
deployment's administration, and the operator's instinct -- "restart it with
`LCATD_BOOTSTRAP_ADMIN` set" -- silently does nothing.

Every other destructive path in libcat has a guard: authorities refuse a
self-merge (tasks/200), batch refuses an empty or whitespace selection
(tasks/205), profile saves are validated before they persist. This one has none.

## Expected

1. `SetRoles` refuses to remove the last `RoleAdmin` -- 409 with something like
   `cannot remove the last admin`. Same for `DeleteUser` on the last admin.
2. The handlers reject caller == target for the role change and the delete
   (403 `admins cannot change their own role`), so a slip needs a second admin
   to be recoverable even when more than one exists.
3. Failing 1 and 2, make `Bootstrap` actually restore: on `ErrUserExists`, if
   the existing user lacks `RoleAdmin`, re-grant it and log loudly. Then the
   documented hatch works and the lockout is a reboot rather than a store edit.

(1) alone fixes the unrecoverable case; (1)+(2) is the belt-and-braces version.

## Repro

```
cd ~/libcat-e2e && node harness/probe_users.mjs   # U8, U9, U11, U14 FAIL
cd ~/libcat-e2e && node harness/retest.mjs        # reports 207 STILL-BROKEN
```

Both run entirely against a sentinel admin (`zz-e2e-admin@example.org`), created
and deleted by the probe. `eve@example.org` is never demoted or deleted, and the
probe asserts at the end that she still holds `["admin"]`.

## Not bugs (verified clean this cycle)

The rest of the users surface is sound: `/v1/users` is admin-only (anon -> 401,
librarian -> 403 on list, create, setRoles and delete); create validates the
email, a minimum 8-character password, and role names, and persists nothing when
it rejects; a duplicate create -> 409; `setRoles`/`delete` on an unknown user ->
404; an unknown role -> 400. Deleting a user kills their session properly --
`Refresh` (`local.go:161`) re-reads the user record, so both login and refresh
return 401 afterwards. The 15-minute window where a stale access token still
carries its old roles is the ordinary stateless-JWT tradeoff and is disclosed at
`users.go:151` ("their refresh tokens die at next use").

`PUT roles {"roles":[]}` leaves a user with no roles at all (204). They can still
log in but every guarded route returns 403. Harmless, but worth deciding whether
an empty array should be rejected rather than accepted.

## Outcome

All three Expected items shipped (cd076f0 + c446986, released
v0.59.0) -- the belt-and-braces version plus the working hatch:

1. SetRoles/DeleteUser refuse removing the last admin (ErrLastAdmin ->
   409 "cannot remove the last admin"). The read-then-write window
   between concurrent demotions of two DIFFERENT admins is accepted
   and documented -- the guard targets the fat-fingered request.
2. Handlers reject caller==target for role changes AND deletes (403
   "admins cannot change their own role" / "…delete their own
   account").
3. Bootstrap actually restores: on ErrUserExists with a user lacking
   admin, it re-grants and appdeps logs loudly. Unit test simulates
   the pre-guard lockout via a raw store delete (the guard itself now
   prevents creating that state through the API).

Verified live: self-demote/self-delete -> 403; delete-by-another-admin
-> deleted user's login 401 (your U12 semantics hold). Probe note:
U8/U9/U11 now PASS; U12/U14 report FAIL as sequencing artifacts of the
fix -- U12's "deleted user" is never deleted because the self-delete
it depended on is refused, and U14 probes the boot-time hatch through
the API 409 (correct behavior, unobservable reboot). The hatch is
covered by TestBootstrapRestoresDemotedAdmin. Your "roles: []" note
was left as-is (offboarding-without-delete is arguably legitimate) --
flag it if you want a 400.
