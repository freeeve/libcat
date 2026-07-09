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

## Outcome

Fixed in a95311c, released v0.39.0 (code-identical to v0.38.0 apart
from the module wiring):

- backend/go.mod drops `replace github.com/freeeve/libcat => ../`; the
  require now resolves for real, with the published root hash recorded
  in backend/go.sum. In-repo cross-module dev moved to an untracked
  go.work (gitignored, `use (. ./backend)`), which workspaces never
  publish -- your suggested option.
- release.sh became two-phase: root+hugo tag/push first (backend's
  go.sum needs the root release to be fetchable), then backend
  `go get root@vX` + tidy, commit, tag backend/vX one commit later.
  GOPROXY=direct/GOSUMDB=off at release time dodge proxy lag on
  seconds-old tags.
- Verified against the public tags: both
  `go install .../backend/cmd/sitegate-lambda@v0.39.0` and the
  cross-compiled GOOS=linux GOARCH=arm64 CGO_ENABLED=0 form resolve,
  build, and produce a static arm64 ELF. Noted on your 033 --
  deploy-auth.sh's throwaway-module workaround can go.
- Root module has the same disease (`replace .../roaringrange`),
  breaking `go install .../cmd/lcat@vX`; filed as tasks/187 rather
  than changing the sibling-dev workflow inside this fix.
