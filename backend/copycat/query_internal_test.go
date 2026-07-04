package copycat

import "testing"

// TestSRUQuery pins the CQL the fielded search sends: dc-set indexes, AND
// composition, the Bath-profile identifier indexes (isbn/issn/lccn), and a
// target's per-index overrides winning over the defaults.
func TestSRUQuery(t *testing.T) {
	cases := []struct {
		name   string
		target Target
		terms  []FieldTerm
		want   string
	}{
		{"any", Target{}, []FieldTerm{{Index: "any", Term: "dutch house"}}, `"dutch house"`},
		{"isbn-bath", Target{}, []FieldTerm{{Index: "isbn", Term: "9780062963673"}}, `bath.isbn = "9780062963673"`},
		{"issn-bath", Target{}, []FieldTerm{{Index: "issn", Term: "0028-0836"}}, `bath.issn = "0028-0836"`},
		{"lccn-bath", Target{}, []FieldTerm{{Index: "lccn", Term: "2019005498"}}, `bath.lccn = "2019005498"`},
		{
			"anded",
			Target{},
			[]FieldTerm{{Index: "title", Term: "dutch house"}, {Index: "author", Term: "patchett"}},
			`(dc.title = "dutch house") and (dc.author = "patchett")`,
		},
		{
			"isbn-override",
			Target{Indexes: map[string]string{"isbn": "pica.isb"}},
			[]FieldTerm{{Index: "isbn", Term: "9780062963673"}},
			`pica.isb = "9780062963673"`,
		},
		{
			"override-leaves-others",
			Target{Indexes: map[string]string{"isbn": "dnb.num"}},
			[]FieldTerm{{Index: "title", Term: "dutch house"}, {Index: "isbn", Term: "9780062963673"}},
			`(dc.title = "dutch house") and (dnb.num = "9780062963673")`,
		},
	}
	for _, tc := range cases {
		if got := sruQuery(tc.target, tc.terms).String(); got != tc.want {
			t.Errorf("%s: cql = %s, want %s", tc.name, got, tc.want)
		}
	}
}
