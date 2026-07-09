# 241 -- a raw NUL byte in backend/ui/src/lib/api.ts makes the file binary to grep and git diff

Filed from libcat-e2e on 2026-07-09 (cross-repo ask).

## Symptom

```
$ file backend/ui/src/lib/api.ts
backend/ui/src/lib/api.ts: data

$ grep -n runBatch backend/ui/src/lib/api.ts
$ echo $?
1
```

`grep` finds nothing -- not because the symbol is absent, but because it
classifies the file as binary and stops. `runBatch` is defined at line 516.
With `-a` it appears:

```
$ grep -a -n "runBatch" backend/ui/src/lib/api.ts
516:export function runBatch(req: {
```

The file is valid UTF-8 and valid TypeScript; it compiles, `svelte-check` is
clean, and the SPA works. Only the tooling around it misbehaves.

## Root cause

`backend/ui/src/lib/api.ts:317` uses a literal NUL as a cache-key separator,
typed as an actual `0x00` byte in the source rather than as an escape sequence.
Written with the byte shown as `<NUL>`, the line is:

```
  const key = `${scheme}<NUL>${id}`;
```

Quoting that byte verbatim in this task file would make the task file binary
too -- which is the whole trap, and which I did twice while writing this up.
Offset 11584 is the only control byte in `api.ts`. GNU/BSD `grep`, `git diff`,
`git grep`, and most editors and review tools treat any file containing a NUL
as binary.

## Why it matters

Small, but it silently subverts search, which is how everyone navigates this
file -- and `api.ts` is the client's single API surface, 870 lines, the file a
person is most likely to grep.

The failure mode is the dangerous kind: `grep` exits 1 and prints nothing, which
reads as "the symbol does not exist" rather than "I did not look". I lost real
time to exactly this while probing the bulk-edit path -- I concluded from an
empty `grep batch backend/ui/src/lib/api.ts` that the SPA never calls a batch
endpoint, and went off to test `POST /v1/batch` instead of `POST /v1/batch/ops`,
which is the route the Bulk Ops screen actually uses. The report I was drafting
would have been about the wrong endpoint.

`git diff` also renders the whole file as `Binary files differ`, so any change
to `api.ts` is unreviewable in a terminal diff and in most web review UIs.

## Expected

Write the separator as an escape sequence rather than as a raw byte: the six
source characters backslash, `u`, `0`, `0`, `0`, `0`, inside the template
literal where the byte is now. The runtime string is identical and the source
stays plain text.

That form is preferable to a bare backslash-zero, which is legal in a template
literal but can be misread as the start of an octal escape. A printable
separator that cannot occur in a scheme or an id (`"|"`, say) would read better
still; changing the key shape only invalidates the in-memory `termCache`, which
is harmless.

Worth adding a guard, since nothing catches this today: a check that no tracked
text file contains a control byte outside tab, CR and LF. Marking `*.ts` as
`text` in `.gitattributes` fixes `git diff` but not `grep`, so the check is the
part that matters.

## Repro

```
file backend/ui/src/lib/api.ts                 # -> "data", expected a text type
grep -c runBatch backend/ui/src/lib/api.ts     # -> silent, exit 1
grep -c -a runBatch backend/ui/src/lib/api.ts  # -> 1
```

Fixed when `file` reports a text type and a plain `grep` finds the symbol.
`harness/retest.mjs` does not cover this one: it is a repo-hygiene defect, not a
runtime behaviour, and the harness only talks to the running playground.

## Outcome

Fixed in **v0.94.0**.

```
$ file backend/ui/src/lib/api.ts
backend/ui/src/lib/api.ts: Java source, ASCII text
$ grep -c runBatch backend/ui/src/lib/api.ts
1
```

Took the printable separator (`"|"`), which cannot occur in a scheme or an IRI,
rather than the escape sequence -- the report's own preference, and it removes
the class of mistake instead of re-encoding it. Only the module-lifetime
`termCache` is invalidated by the key shape changing.

## The guard found a second one immediately

`internal/hygiene.TestNoControlBytesInTextFiles` scans every tracked text file
for a control byte outside tab, CR and LF. On its first run it failed on a file
nobody had noticed:

```
backend/ui/src/a11y.test.ts: control byte 0x1f at offset 27763
```

A duplicate-group fixture had two raw unit separators in its cluster key, because
`identity.keySep` really is the unit separator. The Go source writes that as an
escape and stays plain text; the TypeScript fixture had the byte typed in. It now
builds the separator from its code point (`String.fromCharCode(0x1f)`), so the
fixture keeps its fidelity and the file keeps being greppable.

Verified the guard fails on a planted control byte and passes once it is removed,
rather than trusting a green run.

## Then the guard caught this very file

Writing the section above, I described the escape sequence by typing it -- and my
editing tool resolved it to the character it denotes. The guard failed on
`tasks/241`, at the sentence explaining how the byte gets in.

The filer wrote "which I did twice while writing this up." Three, now.

Both original bytes arrived the same way: an escape sequence in an editor's
replacement text is resolved to a character before it reaches the file, so the
escape one means to write becomes the byte one is trying to avoid. That is also
why the fix cannot be applied with the same tool -- matching the line requires
putting the control byte into the search text, which has the same problem. The
`a11y.test.ts` substitution was made with `perl -pi`, deliberately and once, and
this file was rewritten whole.

The lesson generalizes past this repo: **a fixture that must contain a control
character should construct it, never contain it.** And a guard is worth more than
a resolution to be careful -- I was being careful, in a file about being careful,
and still wrote the byte.
