# 138 -- editor UI: show contributor roles; sort the primary contribution
# first

Filed from queerbooks-demo (2026-07-06, Eve's report while cataloging). Do
not let a queerbooks session edit this repo -- implement here.

## Symptom

Record editor, work w00hkr3k5ljsjm ("Frankie & Bug", coll corpus): the
CONTRIBUTORS panel shows

    Channing, Stockard   coll
    Forman, Gayle        coll

-- no roles, and the narrator above the author.

## The data is complete; this is display-only

The editor API returns the raw grain, and the grain carries everything:

    _:c14n11 a bflc:PrimaryContribution ; bf:agent [Forman]  ; bf:role [rdfs:label "author"] .
    _:c14n3  a bf:Contribution         ; bf:agent [Channing] ; bf:role [rdfs:label "narrator"] .

The SPA renders only the agent label: it ignores the contribution's bf:role
node and the PrimaryContribution type. (The hugo module gets both right --
work cards show "(author)" and order primary-first -- so this is
backend/ui's native-view renderer, not the crosswalk.)

## Suggested shape

- Render the role label(s) after the name, muted -- "Channing, Stockard
  (narrator)" -- matching the public site's presentation.
- Order: PrimaryContribution first, then grain order; today it appears to be
  blank-node-label order, which is arbitrary post-canonicalization.
- While in there: the role should probably be editable like other fields,
  but display-only is the fix this task needs.
