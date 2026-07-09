# 186 -- nquads creator fallback must apply the contributor junk and length policy (coll-support 029)

Filed from coll-support on 2026-07-08 (cross-repo ask).

Found in queerbooks' post-030-flip parity residue (their scripts/collnq-parity.py,
grain w4lardjvsir0oq = coll:32780): when a record carries NO contributor
statements, the 182 creator fallback fabricates a Contribution from the
creator literal -- but without the junk/length policy the mapped-contributor
path (and the old qbd provider's contributionsFromAuthor) applies. coll:32780's
creator is a 158-byte Mongolian conference name; coll-support's export
correctly drops it as a contributor (maxContributorName = 100 bytes: an
overlong "name" is debris/a list, and its term slug overflows Hugo's
255-byte filename limit), yet the flipped grain GAINS a PrimaryContribution
labeled "(Conference), \"Zhendėrėės ..." -- lastFirst applied to the raw
access point.

Fix: the creator fallback should run the same gate as parsed contributors --
drop when the name exceeds the length bound or matches the junk patterns
(© lines, "all rights reserved", year-led, "copyright holder" role). A
record whose every agent is junk should yield a Work with no contributions,
exactly as the coll provider did. The creator literal itself must keep
feeding the identity author key (that path is unaffected by the drop).

Repro: ingest coll-support's catalog.coll.nq with the 030 mapping and diff
grain w4lardjvsir0oq against queerbooks' works-qbd-pre030flip snapshot.

## Outcome

Fixed in 35611ea, released v0.40.0. ingest/nquads ports the coll
provider's gate (isJunkContributor + maxContributorName=100 bytes,
byte-for-byte the coll-support parse.go policy) and runs it on BOTH
contribution paths -- mapped contributor literals (defense against
feeds that don't pre-gate per the coll-feed contract) and the 182
creator fallback:

- An all-junk record yields a Work with no contributions; all-junk
  mapped contributors fall back to a clean creator, matching the old
  qbd precedence.
- The raw creator literal still feeds the identity author key
  (Identity() reads w.creators directly; untouched).
- Also fixed en passant: the fallback's Primary flag tracked creator
  index 0, so a junk first creator left NO primary agent; it now
  tracks the first survivor.

Repro run as specified: full catalog.coll.nq (62,602 grains) ingested
with queerbooks' coll-feed-mapping.toml. coll:32780's grain no longer
carries any Contribution and diffs clean against the qbd
works-qbd-pre030flip snapshot after id normalization, save the
whitelisted 028 @lang prefLabel gains. Filed the rebuild note to
queerbooks-demo (their parity residue drops to the 4 read-race grains
coll-support already fixed).
