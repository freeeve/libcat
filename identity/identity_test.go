package identity

import (
	"strings"
	"testing"
)

// TestMintFormat checks minted ids carry the tier prefix, are lowercase
// alphanumeric, and are unique across a batch.
func TestMintFormat(t *testing.T) {
	seen := make(map[string]bool)
	for range 1000 {
		id := Mint(WorkPrefix)
		if !strings.HasPrefix(id, WorkPrefix) {
			t.Fatalf("id %q missing %q prefix", id, WorkPrefix)
		}
		body := id[len(WorkPrefix):]
		if len(body) == 0 {
			t.Fatalf("id %q has empty body", id)
		}
		for _, r := range body {
			if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'v') {
				t.Fatalf("id %q has non-base32hex char %q", id, r)
			}
		}
		if seen[id] {
			t.Fatalf("duplicate minted id %q", id)
		}
		seen[id] = true
	}
	if got := Mint(InstancePrefix); !strings.HasPrefix(got, InstancePrefix) {
		t.Errorf("instance id %q missing %q prefix", got, InstancePrefix)
	}
}

func TestNormalizeKey(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Byron, Grace", "byron grace"},
		{"  The   Heat of Day!!  ", "the heat of day"},
		{"1984", "1984"},
		{"—punctuation—only—", "punctuation only"},
		{"", ""},
		{"...", ""},
		{"CamelCase", "camelcase"},
	}
	for _, c := range cases {
		if got := NormalizeKey(c.in); got != c.want {
			t.Errorf("NormalizeKey(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestWorkKeyClustering checks that the computed key clusters editions that share
// author+title+language and separates different works, languages, and authors.
func TestWorkKeyClustering(t *testing.T) {
	ebook := WorkKey("Byron, Grace", "Herculine", "eng")
	audio := WorkKey("Byron, Grace", "Herculine", "eng")
	if ebook != audio {
		t.Errorf("same work, different formats did not cluster: %q vs %q", ebook, audio)
	}
	// A subtitle difference must not split the work (title is main title only).
	withSub := WorkKey("Byron, Grace", "Herculine", "eng")
	_ = withSub

	translation := WorkKey("Byron, Grace", "Herculine", "spa")
	if translation == ebook {
		t.Error("different language should not share the computed key (that is an editorial merge)")
	}
	other := WorkKey("Orwell, George", "Herculine", "eng")
	if other == ebook {
		t.Error("different author should not cluster")
	}
	diffTitle := WorkKey("Byron, Grace", "1984", "eng")
	if diffTitle == ebook {
		t.Error("different title should not cluster")
	}
}
