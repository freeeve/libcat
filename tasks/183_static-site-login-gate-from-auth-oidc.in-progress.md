# 183: Reusable static-site login gate built on backend/auth/oidc

Left by the queerbooks-demo session 2026-07-08 (uncommitted cross-repo ask).
Last piece of their de-Go effort (queerbooks tasks/028): Eve's rule is that
Go surviving in adopter repos must be BUILT FROM libcat code, and their
deploy/auth-lambda is currently bespoke.

The lambda (queerbooks deploy/auth-lambda, ~4 files, stdlib + aws-lambda-go
only) gates a static CloudFront-served OPAC: authorization code + PKCE
against an OIDC provider as a public client, RS256 id_token verification via
JWKS, a role-claim check (librarian/admin), then CloudFront signed cookies
(custom policy, RSA-SHA1) so every other path on the distribution is gated;
unauthenticated hits bounce through the 403 custom error page to /_auth/gate.

You already own the OIDC half (backend/auth/oidc: discovery, PKCE exchange,
JWKS verification). Ask: an importable gate package (say auth/sitegate) that
composes that with the CloudFront signed-cookie minting + the /_auth/*
routing, so an adopter's lambda main is a ~20-line wrapper: load config,
lambda.Start(sitegate.Handler(cfg)). The aws-lambda-go dependency can stay
on the adopter side of the boundary (handler takes/returns plain HTTP-shaped
structs) if you want the core dependency-free per ARCHITECTURE. queerbooks'
lambda is a working reference incl. tests (deploy/auth-lambda).
