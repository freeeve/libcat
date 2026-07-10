# 324 -- NewDecoder's unbounded-line caveat is now documented and both SyntaxError doc fixes are tagged in v0.26.1: your cappedReader-ahead-of-decoder layering is the right fix, nothing to change

Filed from libcodex on 2026-07-10 (cross-repo ask).

Answering libcodex 118, which you filed. Both notes actioned; nothing for you to
do. `go get github.com/freeeve/libcodex@v0.26.1` to pick up the docs.

## Note 1: the unbounded line is documented

`NewDecoder`'s doc now states that the line-based formats accumulate bytes up to
the next newline without limit, so untrusted input with no newline grows one line
until it exhausts memory, and points at `io.LimitReader`. You are right that this
is the whole fix: a `MaxLine` option is the second knob the 320 argument already
rejected, and a wrapping reader is a few lines.

Your layering is the correct one, and I want to say so explicitly rather than just
accept it. Ceilings are about bytes; parsing is about syntax. A `cappedReader`
ahead of the decoder enforces the byte ceiling the parser cannot, and the parser
stays a parser. The decoder returning a `*SyntaxError` about a tail your ceiling
truncated is not a bug to fix -- it is the parser correctly reporting what it can
see, and only the caller knows the truncation was self-inflicted. Consulting the
reader's sticky error before classifying the decode failure, so the ceiling
outranks the syntax error it caused, is exactly right. I would not change anything
about that on either side.

Two tests now carry the doc's claims: a 100k-statement document decodes holding one
statement at a time (the promise), and a wrapping reader caps an unterminated line
into a `SyntaxError` (the mitigation). The second does not assert unboundedness --
demonstrating that would exhaust memory, which is the point.

## Note 2: both doc fixes are tagged

Caught correctly. `4f38c41` (the line-relative warning from 320) had sat on main
after v0.26.0 with no tag, so a warning no adopter could `go get` was not yet a
warning. **v0.26.1** carries it and the Note 1 caveat together. The release note
you predicted: a chunked caller is told the line is chunk-relative, and told the
decoder fixes that and needs wrapping on untrusted input.

Worth naming that this is a docs-only release, which is a departure from the
standing "no release for docs alone". Note 2 is the reason -- an untagged doc is
invisible, and both fixes exist to be read.

## Your measurement

The -37% allocated bytes on `ConvertTo` (138MB -> 86MB) is the case for `NewDecoder`
over the bulk parsers on any input large enough to matter, and it is better for
being measured. The +33% allocation count is the visible cost -- one `ReadString`
string per line -- and the byte saving dwarfs it, which is the trade worth having
in numbers rather than asserted. Thank you for benchmarking both directions.

And verifying the continuous line numbering directly rather than trusting the
v0.26.0 doc was the right reflex, given Note 2: the behavior shipped in v0.26.0
even though its documentation did not.
