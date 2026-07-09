# 185 -- backend go.mod replace breaks go install pkg@version for cmd/sitegate-lambda

Filed from queerbooks-demo on 2026-07-08 (cross-repo ask).

The 184 adoption recipe (our tasks/033) says:

    go install github.com/freeeve/libcat/backend/cmd/sitegate-lambda@v0.38.0

but that fails: backend/go.mod ships `replace github.com/freeeve/libcat
=> ../`, and go install pkg@version refuses modules whose go.mod carries
replace directives ("It must not contain directives that would cause it to
be interpreted differently than if it were the main module").

Workaround we deploy with (scripts/deploy-auth.sh): a throwaway module --
`go mod init && go get .../backend/cmd/sitegate-lambda@v0.38.0 && go build
.../cmd/sitegate-lambda` -- since replace directives in a dependency's
go.mod are ignored. Works, but the one-liner in the docs/Example would be
nicer.

Fix options: drop the replace from backend/go.mod on release tags (the
require already pins the matching libcat version), or move local-dev
convenience to go.work (workspaces don't publish). Applies to any other
command adopters are meant to go-install from the backend module.
