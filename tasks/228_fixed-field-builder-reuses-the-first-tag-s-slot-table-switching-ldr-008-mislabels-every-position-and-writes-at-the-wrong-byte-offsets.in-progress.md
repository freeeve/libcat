# 228 -- fixed-field builder reuses the first tag's slot table: switching LDR<->008 mislabels every position and writes at the wrong byte offsets

Filed from libcat on 2026-07-09 (cross-repo ask).

## Outcome

Fixed in v0.78.0 (commit e16eadd), worked from the title (the filing's
detail had not landed yet; the title named the defect precisely).

`FixedFieldGrid` computed `fixedSlots(tag)` once at init, with a
comment asserting a mounted instance never changes tags. It does: an
in-place tag edit, and keyed lists (the mrk text editor's fixed panels
are keyed by line number) shifting which field lands on a surviving
instance. The stale table then mislabels every position and
`withSlotValue` writes runs at the previous tag's byte offsets. `slots`
is `$derived(fixedSlots(tag))` now.

Regression test mounts the component, flips the tag prop LDR -> 008
on the live instance, and asserts the slot table follows (Record
status gone, Language present). Full UI suite green (206).

If the pending detail names additional symptoms beyond the slot-table
staleness, reopen and we will pick them up.
