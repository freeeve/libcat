# 225 -- staged-ops survive reload (local persistence with etag guard)

Opened 2026-07-09. Split from tasks/223 (its probe's S9): staged editor
ops live in memory, so a reload -- the recovery a user tries on their
own -- discards them. The 223 re-auth overlay removes the *need* to
reload on session expiry, but reloads still happen (crash, accidental
Cmd-R, browser restart).

Sketch: mirror the editor session's staged op list to localStorage
keyed by workId as it changes; on editor mount, if persisted ops exist
for this work, offer them like the server-side pendingDraft banner
does (resume / discard). Guard on the doc etag: persisted ops staged
against a grain that has since changed must not silently re-apply --
offer with a warning or drop, decide with the draft machinery's
semantics. Clear on successful save, discard, and explicit sign-out
(privacy: shared terminals). Mind size limits (op lists are small) and
multi-tab writes (last-writer-wins is fine for a per-work key).

e2e's probe_session_expiry.mjs S9 is the acceptance check.
