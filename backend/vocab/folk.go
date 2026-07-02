package vocab

import (
	"errors"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// FolkScheme is the reserved scheme key for folksonomy tags -- community
// terms that are not (yet) in any controlled vocabulary. Folk terms carry
// their normalized text as the TermRef ID.
const FolkScheme = "folk"

// TermRef points at a suggestible term: a controlled-vocabulary concept
// (Scheme = a loaded vocabulary key, ID = authority URI) or a folksonomy tag
// (Scheme = FolkScheme, ID = normalized tag text). Label is display-only and
// never trusted for identity.
type TermRef struct {
	Scheme string `json:"scheme"`
	ID     string `json:"id"`
	Label  string `json:"label,omitempty"`
}

const (
	folkMinLen = 2
	folkMaxLen = 60
)

// ErrBadFolkTerm reports a raw tag the normalizer refuses.
var ErrBadFolkTerm = errors.New("vocab: unusable folksonomy term")

// NormalizeFolk canonicalizes a raw community tag: Unicode NFKC, lowercase,
// whitespace collapsed, length-bounded, with control characters, URLs, and
// markup rejected outright. The result is the tag's identity (dedup key and
// TermRef ID); raw patron text itself never reaches the graph -- a normalized
// novel term still has to pass moderation before it becomes suggestible.
func NormalizeFolk(raw string) (string, error) {
	// Collapse whitespace (tab/newline are separators, not content) before
	// the control-char check, so only embedded controls reject.
	s := strings.Join(strings.Fields(norm.NFKC.String(raw)), " ")
	for _, r := range s {
		if unicode.IsControl(r) {
			return "", ErrBadFolkTerm
		}
	}
	// Lowercase and NFKC interact both ways (case folding can denormalize;
	// normalization can surface uppercase compatibility forms like
	// U+03D2 -> Υ), so iterate the pair to a fixpoint. Rejects the rare
	// pathological input that will not settle.
	settled := false
	for range 4 {
		next := norm.NFKC.String(strings.ToLower(s))
		if next == s {
			settled = true
			break
		}
		s = next
	}
	if !settled {
		return "", ErrBadFolkTerm
	}
	if n := len([]rune(s)); n < folkMinLen || n > folkMaxLen {
		return "", ErrBadFolkTerm
	}
	for _, banned := range []string{"://", "www.", "<", ">", "&#", "\\u", "{", "}"} {
		if strings.Contains(s, banned) {
			return "", ErrBadFolkTerm
		}
	}
	return s, nil
}
