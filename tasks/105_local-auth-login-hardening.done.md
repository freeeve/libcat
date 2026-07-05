# 105 -- auth/local: login hardening (timing oracle, leaking rate counters, index repair)

> Filed from the 2026-07-05 full-code review. Four related defects in
> backend/auth/local.

## 1. Login is a timing oracle for account enumeration (security, medium)

For an unknown email, `Login` returns `ErrBadCredentials` right after `getUser`
fails (local.go:105-109) -- no hash is computed. For a known email with a wrong
password, `verifyPassword` runs argon2id (64 MiB) -- tens of ms. Latency
distinguishes valid staff emails from invalid ones with one request each, which
defeats the stated goal at local.go:29-31. Fix: verify against a fixed dummy
hash on the unknown-user path so both branches cost the same.

## 2. Rate-limit pre-read creates permanent TTL-less items (correctness/cost, medium)

`Login` reads the counter with `Increment(ctx, rateKey, 0, time.Time{})`
(local.go:101). In the Dynamo store an empty `expireAt` skips the `SET #exp`
clause (store/dynamo/dynamo.go:241-245), so every clean first-try login creates
a `cnt=0` item with no TTL -- one orphaned item per (email, hour-window),
forever, contradicting the "TTL clears stale windows" comment at
local.go:118-119. Fix: pass the TTL on the read-path Increment too, or drop the
pre-read and use `recordFailure`'s return value.

## 3. Per-email failure cap enables lockout DoS (security, low)

The counter key `RATE#LOGIN#<email>` has no attacker dimension; anyone knowing a
staff email can send 10 bad passwords and lock the real user out until the
calendar hour rolls (local.go:99-104). Decide whether this deployment needs a
per-IP dimension or an admin unlock path; document the trade-off either way.

## 4. CreateUser profile/index writes are non-atomic (correctness, low)

`CreateUser` Puts the profile (`CondIfAbsent`) then the `USERS` index item
separately (users.go:87-94). If the index Put fails, a retry hits
`ErrUserExists` and never repairs the index -- the user exists but is invisible
to `ListUsers` forever; `Bootstrap` maps `ErrUserExists` to nil (users.go:187-189)
so a re-run doesn't repair it either. Fix: on `ErrUserExists`, idempotently
re-assert the index item.

## Acceptance

- Unknown-email and wrong-password logins are indistinguishable by timing
  (dummy-hash verify on both paths).
- No login path writes a store item without a TTL; a soak of successful logins
  leaves no residual RATE items past expiry.
- CreateUser retried past a simulated index-write failure ends with the user
  listed.
- Lockout-DoS decision recorded (fix or accepted trade-off).

## Resolved

1. Unknown-user logins now verify against a fixed, well-formed
   `dummyPasswordHash` (same argon2id parameters as `hashPassword`), so both
   failure branches burn the same work. `TestDummyHashBurnsRealWork` guards
   that the constant parses -- a malformed constant would shortcut before the
   argon2 call and silently reintroduce the oracle.
2. The login pre-read (`Increment` delta 0) now stamps the shared
   `rateWindowTTL` (2h), so clean logins no longer leave permanent counter
   items. `TestRateWindowExpires` steps the store clock past the TTL and
   asserts the window is gone.
3. Lockout DoS: accepted trade-off, documented at the `loginFailureCap`
   declaration -- per-account keying stays; caller-keyed limits belong at the
   edge proxy, and locked windows self-clear via the counter TTL.
4. `CreateUser` re-asserts the `USERS` index item on the `ErrUserExists` path,
   so a create (or `Bootstrap` re-run) repairs a torn profile/index write.
   `TestCreateUserRepairsIndex` covers hide-then-repair via `ListUsers`.
