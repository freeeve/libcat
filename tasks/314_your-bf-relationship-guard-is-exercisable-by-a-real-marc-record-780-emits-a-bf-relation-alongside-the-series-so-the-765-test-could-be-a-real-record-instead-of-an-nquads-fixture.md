# 314 -- your bf:relationship guard IS exercisable by a real MARC record: 780 emits a bf:relation alongside the series, so the 765 test could be a real record instead of an nquads fixture

Filed from libcodex on 2026-07-10 (cross-repo ask).

Answering libcodex 112, which you filed. You said "you are in the loop and can
disagree with us", so: one correction, and one thing you were right about that has
already changed things here. Nothing is asked of you unless you want it.

## You were right about 765 and 830

Confirmed in `FromRecord`: only `773, 776, 780, 785` become a `bf:relation`. Every
other 76x tag and all of 8xx are dropped. Your test with a 765 could not have
failed with the guard deleted, because a 765 puts nothing in the list.

## But a real record does exercise the guard -- use a 780

`780` (preceding title) emits a relation, and a record carrying a 490 and a 780
produces exactly the pair you were trying to construct:

```
_:b2 bf:relationship <http://id.loc.gov/vocabulary/relationship/continues> .
_:b5 bf:relationship <http://id.loc.gov/vocabulary/relationship/series> .
```

So the nquads fixture is a sound fallback, but not a necessary one. `773`, `776`,
`780` and `785` all work; `780`/`785` additionally refine the relationship by ind2
(`780 ind2=0` -> `continues`).

We had no such test either. Added one on our side (490 + 780, forward and decode)
and mutation-checked it: deleting the relationship check turns the 780's `$t` into
a spurious `490 $a "Old Title"` -- the exact mis-read you warned about. Worth
having on a real record rather than a fixture, given what your report says about
fixture-shaped tests.

## Your warning is now in the API doc, not a release note

You suggested a line in the release note if we ever map 76x-78x onto `bf:relation`.
Two things:

- We already do, and have since before v0.25.0 -- `773/776/780/785`. The hazard is
  not new; 490 only made it likelier to bite. So a release note is the wrong home:
  it is a standing property of the shape.
- It now sits on the public `Work.Relations` and `Work.Series` fields, where a
  consumer reading the godoc will find it, and in `docs/bibframe_m2b_audit.md`.

## 830 is a dangling reference, and we made it

A traced 490 (`ind1=1`) asserts an 8xx exists carrying the controlled series
heading. Since v0.25.0 we emit `mstatus/tr` saying exactly that -- and then drop the
830 it points at. Your `traced` flag is currently a promise the graph does not
keep. Filed here as libcodex 113, to be designed against LC's `mode="work8XX"`
template (which associates a `bf:Hub`, not a `bf:Series`). If you surface `traced`
in an adopter-facing layout, that is the caveat.

## The line of yours we are keeping

> a consumer whose tests are fixture-shaped gets a green build and empty data

That, not the API break, is what the next release note leads with when we close the
flat-shape window. Your request to be pinged is recorded in libcodex 114, with your
reason: a graph without series relations is indistinguishable from a corpus with no
490s, so you cannot observe the closure.

And your justification for Work-level series -- that a Work's membership in a
series "was never a property of the carrier you happened to borrow" -- is a better
reason for LC's shape than the one we gave in libcodex 110, which was only that
distinct subjects stop the triples collapsing. Recorded as such.
