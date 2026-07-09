# 255 -- deleting a vocab source with an installed snapshot leaves an orphan row whose Upload and Delete buttons 404

Filed from libcat-e2e on 2026-07-09 (cross-repo ask).

Found while probing the download job lifecycle end to end (that half is healthy --
16/16, see the Outcome of the exploration in `libcat-e2e/ADMIN_FEATURES.md`).

## Symptom

On the Vocabularies screen, **Delete source** carries this tooltip:

> "Delete this registered source definition (an installed snapshot must be removed
> first)"

Nothing enforces that. Clicking it while a snapshot **is** installed succeeds silently
-- no error, no confirmation -- and the row becomes an *orphan install*: the terms are
still in the index, the row still lists them, and two of its three buttons can no
longer work.

Measured through the real UI on :8481, with a sentinel source whose snapshot was
downloaded from a local origin:

```
sentinel row buttons          : ["Refresh","Remove","Upload…","Delete source"]
click "Delete source" (installed):
  status/error text           : ""            <- silent success
  orphan row buttons          : ["Remove","Upload…","Delete source"]
click "Delete source" (orphan):
  error shown                 : "no such source"
  row still present           : true
```

And directly against the API, on the same orphan:

```
orphan view: {"name":"zz-e2e-orph","scheme":"zz-e2e-orph",
              "installed":{"terms":1,"installedAt":"…","snapshotUrl":"http://127.0.0.1:…/ok.nt"}}

PUT    /v1/vocabsources/zz-e2e-orph/snapshot   ("Upload…")      -> 404 {"error":"no such source"}
DELETE /v1/vocabsources/zz-e2e-orph            ("Delete source")-> 404 {"error":"no such source"}
DELETE /v1/vocabsources/zz-e2e-orph/snapshot   ("Remove")       -> 200 {"removed":true}
```

Only **Remove** works -- exactly what `Views` promises ("Synthesized from the sidecar
so the vocabulary stays visible and removable"). The other two are rendered anyway.

The template demonstrably *can* gate these actions: the builtin `lcsh` row already
hides Delete source.

```
builtin lcsh row buttons : ["Refresh","Remove","Upload…"]
sentinel row buttons     : ["Refresh","Remove","Upload…","Delete source"]
```

## Root cause

Three pieces, none of which knows about the other two.

**1. Nothing enforces the tooltip's precondition.**
`backend/vocabsrc/vocabsrc.go:225-236` deletes the registry record regardless of
install state:

```go
func (s *Service) DeleteSource(ctx context.Context, name string) error {
	err := s.DB.Delete(ctx, store.Record{Key: sourceKey(name)}, store.CondNone)
	if errors.Is(err, store.ErrNotFound) {
		for _, b := range Builtins() {
			if b.Name == name {
				return fmt.Errorf("%w: %q is builtin; override it instead", ErrValidation, name)
			}
		}
		return ErrNotFound
	}
	return err
}
```

It checks for a builtin. It never checks `metaPath(name)` for an installed snapshot.

**2. An orphan is indistinguishable from a registered source over the wire.**
`vocabsrc.go:400-407` synthesizes the orphan view, and `SourceView` (`:357-361`)
carries no marker:

```go
for name, info := range byName {
	if !registered[name] {
		views = append(views, SourceView{Source: Source{Name: name, Scheme: info.Scheme}, Installed: &info})
	}
}
```

```go
type SourceView struct {
	Source
	Installed *InstallInfo `json:"installed,omitempty"`
	Job       *Job         `json:"job,omitempty"`
}
```

The client sees `{name, scheme, installed}` and has no way to learn that no source
record backs it. (`snapshotUrl` being empty is not a proxy: a legitimately registered
upload-only source has no snapshot URL either -- that is the documented
`InstallUpload` escape hatch.)

**3. The row renders both actions unconditionally.**
`backend/ui/src/screens/VocabSources.svelte:268-272` -- Upload… is gated only on the
viewer being an admin:

```svelte
{#if isAdmin && !readOnly}
  <label class="button button--quiet upload-btn" …>Upload… <input type="file" … /></label>
{/if}
```

and `:274-279` -- Delete source correctly excludes builtins, and nothing else:

```svelte
{#if isAdmin && !s.builtin && !readOnly}
  <button … onclick={() => void unregister(s)}
    title="Delete this registered source definition (an installed snapshot must be removed first)">
    Delete source
  </button>
{/if}
```

Both then hit `GetSource` on the server (`vocabsources_handlers.go:84` →
`InstallUpload` → `GetSource`, and `:55` → `DeleteSource`), which returns `ErrNotFound`
→ 404 `"no such source"`.

## Why it matters

Orphan installs are not an exotic state -- `Views`' own comment lists two ordinary ways
in besides this one: an offline `lcat vocab-install` (tasks/163), and a deployment
whose registry reset because it has no document store. In every case the admin is
looking at a row that says *"lcsh — 513,125 terms"* and offering them an **Upload…**
button, which is precisely the documented escape hatch for "the publisher's download
URL is unreachable" (`download.go:223-227`). They click it, choose a 512MB dump, and
get `no such source` for a vocabulary the same screen is listing.

The one-click path here makes it worse: the tooltip tells the admin the server will
stop them from deleting a source with a snapshot installed, so a careful admin has been
told they cannot reach this state by accident. They can, silently, in one click.

Nothing is lost -- **Remove** still uninstalls, and re-registering the same name
re-adopts the install -- so this is a usability and honesty defect, not data loss. (It
does compound **252**: the sidecar artifacts survive `Remove`, so an orphan that is
removed still leaves eight files behind.)

## Expected

Both halves, ideally:

- **Enforce the order the tooltip already states.** `DeleteSource` should refuse with a
  `409` (or `400`) when `metaPath(name)` exists: *"remove the installed snapshot
  first"*. That makes the tooltip true and closes the one-click path.
- **Mark orphans, and hide the actions they cannot perform.** Add a flag to
  `SourceView` -- `Orphan bool \`json:"orphan,omitempty"\`` set in the synthesis loop,
  or the inverse `registered` -- and gate `Upload…` and `Delete source` on it in
  `VocabSources.svelte`. Orphans arise from offline installs and registry resets
  regardless of this bug, so the row must render correctly for them either way. A
  "Register this source…" affordance in place of the dead buttons would turn the state
  into a recoverable one rather than a puzzle.

If deleting a source with an install is meant to stay legal, then the tooltip should
say so and say what happens (*"the installed snapshot stays; the row becomes an
uninstall-only orphan"*).

## Repro

```bash
cd ~/libcat-e2e && node harness/probe_vocabdownload.mjs   # D16, D17, D18
cd ~/libcat-e2e && node harness/retest.mjs                # check t255
```

By hand, against :8481 as an admin:

```bash
TOK=…
curl -XPOST -H "Authorization: Bearer $TOK" -H 'Content-Type: application/json' \
  -d '{"name":"zzorph","scheme":"zzorph"}' localhost:8481/v1/vocabsources
printf '<http://example.org/z/1> <http://www.w3.org/2004/02/skos/core#prefLabel> "Z"@en .\n' \
  | curl -XPUT -H "Authorization: Bearer $TOK" --data-binary @- \
      localhost:8481/v1/vocabsources/zzorph/snapshot          # {"installed":true,"terms":1}
curl -XDELETE -H "Authorization: Bearer $TOK" localhost:8481/v1/vocabsources/zzorph
                                                             # 200 -- the tooltip says this cannot happen
curl -s -H "Authorization: Bearer $TOK" localhost:8481/v1/vocabsources | jq '.sources[]|select(.name=="zzorph")'
                                                             # still listed, 1 term, no marker
curl -XDELETE -H "Authorization: Bearer $TOK" localhost:8481/v1/vocabsources/zzorph
                                                             # 404 "no such source"
curl -XDELETE -H "Authorization: Bearer $TOK" localhost:8481/v1/vocabsources/zzorph/snapshot
                                                             # 200 -- the only control that works
```

Or in the UI: open `#/vocabularies`, register a drop-in source, install a dump, then
click **Delete source** on its row. The row survives with `Upload…` and `Delete source`
still offered; both answer `no such source`.
