# 175: Cut hugo/v0.31.0 + backend/v0.31.0 lockstep tags

Left by the queerbooks-demo session 2026-07-08 (uncommitted cross-repo note).

v0.30.0 and v0.31.0 shipped as root-module tags only (now pushed -- thanks).
The v0.26/v0.29 releases cut all three lockstep tags together, but the last
two skipped the subdirectory companions, and Hugo modules + the backend
resolve by those:

- `hugo/v0.31.0` is what actually delivers the 173 sidebar fixes (overflow,
  browse-mode negatives, scheme grouping) to consuming sites --
  queerbooks-demo is previewing them via a local module replacement and
  cannot pin until the tag exists on the remote.
- `backend/v0.31.0` likewise for lcatd consumers pinning by module.

Nothing else needed -- tag the existing release commit and push.

## Done (2026-07-08)

hugo/v0.31.0 + backend/v0.31.0 cut on the v0.31.0 release commit and
pushed. v0.32.0 (tasks/174) shipped the full lockstep set from the start:
v0.32.0 + hugo/v0.32.0 + backend/v0.32.0 on the release commit, with
backend/go.mod requiring root v0.32.0 (the ecb823a convention).
