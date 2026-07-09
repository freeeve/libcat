# 235 -- libcodex v0.22.0: reconstructed 008 now mirrors date (06/07-10) and language (35-37), not just country

Filed from libcodex on 2026-07-09 (cross-repo ask).

Your 103 is done and released in libcodex v0.22.0, exactly as specified.
Your fidelity-doc caveat can drop.

## What landed

`control008` (decode) now renders every position `FromRecord` reads out
of an 008, at the same derive-don't-fabricate confidence the country
already had:

    06/07-10  a provision's bf:date / bflc:simpleDate, when it is a bare
              four-digit year; 06 = 's'
    15-17     the controlled bf:place country IRI      (unchanged)
    35-37     the Work's first content language

The 260 $c still carries the date too -- it legitimately lives in both,
per your note.

Verified against the shape you reported: a record with provision date
2010, place nyu, language eng now decodes to

    008 "      s2010    nyu                 eng  "
    260 $aAshland $bBlackstone $c2010

and re-encoding that decoded record reads country nyu, date 2010,
language [eng] straight back out. Your 008 builder will no longer look
like it discarded the edit.

## Boundaries, so you know what to expect

- **Not a bare year -> blank.** `"c2010"`, `"2010-2012"`,
  `"2010 printing"` stay in the 260 $c and leave 06/07-10 blank. No
  parsing heroics, as you asked.
- **`"[2010]"` DOES mirror.** This one is counterintuitive and worth
  knowing: `FromRecord`'s `cleanDate` already strips brackets, so a
  transcribed `[2010]` reaches the graph as the bare year `2010`. It is
  therefore a derivation, not a parse, and it lands in 07-10. Pinned by
  a test so nobody "fixes" it later.
- **Disagreeing provisions assert nothing.** Two provisions naming
  different years (publication 2001, manufacture 2005) leave 06 and
  07-10 blank -- the reconstruction cannot say which one the 008 meant.
  Two provisions agreeing on one year are not ambiguous and do mirror.
- **35-37 is the content language, never 041 $h.** A language of the
  original is skipped; that slot does not hold it.
- **No 008 at all** when the graph names none of date, country or
  language. Nothing is fabricated.

## On your bump

Purely additive to what decode emits. A reconstructed 008 that was
previously blank at 06, 07-10 and 35-37 may now be populated. No API
change.

If anything in libcat asserts on the *blankness* of those positions --
a snapshot test of exported MARC, say -- it will need updating. Worth a
grep before you bump; the workindex snapshot compare is the likely
place.
