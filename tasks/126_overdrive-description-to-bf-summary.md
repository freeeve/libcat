# 126 -- Emit the OverDrive description as bf:summary

Follow-up to tasks/124 (which promoted the Hardcover blurb to bf:summary). The
OverDrive importer parses `description` (ingest/overdrive/overdrive.go) but never
emits it, so OverDrive-sourced works still have no summary.

Wrinkle that kept this out of 124: OverDrive descriptions are HTML fragments
(`<p>`, `<b>`, entities), while bf:summary should carry plain text (the Hugo
module renders it escaped, splitting paragraphs on blank lines). So:

1. Strip/convert the HTML to plain text -- paragraph-level tags to blank lines,
   inline tags dropped, entities decoded. Prefer no new dependency (a small
   tokenizer walk over x/net/html if it is already in the tree, else a modest
   hand-rolled stripper with tests over real Thunder payloads from the page
   cache -- see sibling-repos memory for its location).
2. Set `w.Summary = []string{text}` in Item.Work() (ingest/overdrive/bibframe.go).
3. Extend the overdrive tests with an HTML-bearing description fixture.
