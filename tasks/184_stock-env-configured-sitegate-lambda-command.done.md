# 184 -- stock sitegate lambda command, toml-configured (like the hugo module)

Filed from queerbooks-demo on 2026-07-08 (cross-repo ask).

Follow-on to 183/sitegate (adopted in our 032 as a ~30-line main). Eve's
de-Go rule for queerbooks-demo (their tasks/028) is zero Go source; the
wrapper main is the last of it. Ask: ship the wrapper as a stock command,
e.g. `backend/cmd/sitegate-lambda`, so adopters build a published command
at a version and write no Go at all.

Config shape: like the hugo module -- driven by a toml section, not a pile
of env vars. Adopters already keep one config file (lcat.toml) driving
build + site; the gate should read a `[sitegate]` table from a toml file
bundled into the deploy zip next to bootstrap (path via SITEGATE_CONFIG,
default ./sitegate.toml), carrying the non-secret sitegate.Config fields:

    [sitegate]
    issuer      = "https://auth.example.org"
    client-id   = "mysite"
    site-domain = "opac.example.org"
    key-pair-id = "K..."
    site-name   = "My Site"          # optional, like min-role, path-prefix,
                                     # session-ttl, scopes, role-claim ...

The secret stays out of the toml: PrivateKeyPEM from env
(e.g. SITEGATE_PRIVATE_KEY_PEM), accepting raw PEM or base64-of-PEM --
raw PEM in env vars is a quoting fight (our deploy base64s it; the
decode-by-prefix helper in our main.go is yours for the taking).

On adoption we delete deploy/auth-lambda (source and go.mod) and the deploy
script becomes: extract/copy the [sitegate] table, cross-compile the stock
command (`GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go install
github.com/freeeve/libcat/backend/cmd/sitegate-lambda@vX`), zip bootstrap +
sitegate.toml.

## Outcome

Shipped `backend/cmd/sitegate-lambda` (ae4ef73, released v0.38.0): the
stock Function-URL gate command; adopters write no Go.

- Config exactly as asked: `[sitegate]` table from SITEGATE_CONFIG
  (default ./sitegate.toml), kebab-case keys mirroring sitegate.Config
  (issuer, client-id, site-domain, key-pair-id; optional site-name,
  min-role, role-claim, role-map subtable, jwks-url, scopes,
  path-prefix, session-ttl as a Go duration string). Optional keys stay
  zero so sitegate.New applies its defaults; role-map values are
  validated against known roles at load (a typo fails startup, not
  silently at login).
- Signer key from SITEGATE_PRIVATE_KEY_PEM, raw PEM or base64-of-PEM
  (decode-by-prefix helper taken from your main.go as offered).
- main is two lines around a testable buildGate; coverage 84%,
  linux/arm64 CGO_ENABLED=0 cross-compile verified.
- Deploy recipe from the ask works as written:
  `GOOS=linux GOARCH=arm64 CGO_ENABLED=0
  go install github.com/freeeve/libcat/backend/cmd/sitegate-lambda@backend/v0.38.0`,
  zip bootstrap + sitegate.toml. Filed the adoption note back to
  queerbooks-demo.
