# 112 -- ui: debounced searches apply stale responses over newer results

> Filed from the 2026-07-05 full-code review.

## Symptom

Type "abc", pause, type "abcd", pause: if the "abc" request resolves after the
"abcd" one, the list repopulates with "abc" results while the box shows "abcd",
and the selection is re-pinned against the wrong list.

## Cause

`search()` in screens/WorkSearch.svelte:53 is debounced but applies responses
unconditionally (`st.works = page.works`) with no abort or generation token
guarding `fetchWorks`. The identical unsequenced pattern exists in
VocabPicker.svelte:89, TagInput.svelte:44, CommandPalette.svelte:92, and
MarcPreviewPane.svelte:66 (its `$effect` refresh).

## Fix sketch

A shared helper: either an `AbortController` cancelled on each new keystroke, or
a request-generation counter checked before applying results (`if (gen !==
latest) return`). Apply it to all five sites.

## Acceptance

- A delayed earlier response can never overwrite a newer query's results (unit
  test with two interleaved mock fetches resolving out of order).
- All five listed components use the shared sequencing helper.
