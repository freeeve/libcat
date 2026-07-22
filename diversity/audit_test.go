package diversity

import (
	"math"
	"testing"
)

// sub is a subject-with-labels test helper.
func sub(uri string, labels ...string) SubjectRef { return SubjectRef{URI: uri, Labels: labels} }

// tally looks up a category tally by id in a report.
func tally(r Report, id string) CategoryTally {
	for _, c := range r.Categories {
		if c.ID == id {
			return c
		}
	}
	return CategoryTally{}
}

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

// TestAuditorCoverageAndCounts checks the coverage-first denominators: a work with
// no usable subject dilutes coverage but is categorized nowhere, and a work is
// counted once per category however many subjects matched it.
func TestAuditorCoverageAndCounts(t *testing.T) {
	a := NewAuditor(Default())

	// 1: two subjects, both LGBTQIA+ -> counted once for lgbtqia; covered.
	a.Add([]SubjectRef{sub("", "Lesbian fiction"), sub("", "Gay men")})
	// 2: immigrant + women -> two categories; covered.
	a.Add([]SubjectRef{sub("", "Immigrants"), sub("", "Women authors")})
	// 3: a subject that maps nowhere -> covered but no category.
	a.Add([]SubjectRef{sub("", "Cooking")})
	// 4: no subjects at all -> dilutes coverage, categorized nowhere.
	a.Add(nil)

	r := a.Report()
	if r.TotalWorks != 4 {
		t.Errorf("TotalWorks = %d, want 4", r.TotalWorks)
	}
	if r.CoveredWorks != 3 {
		t.Errorf("CoveredWorks = %d, want 3 (work 4 has no subjects)", r.CoveredWorks)
	}
	if !approx(r.Coverage, 3.0/4.0) {
		t.Errorf("Coverage = %v, want 0.75", r.Coverage)
	}
	if m := r.Multiplicity; m.Uncategorized != 1 || m.MatchedOne != 1 || m.MatchedMulti != 1 {
		t.Errorf("Multiplicity = %+v, want 1 uncategorized (cooking) / 1 one (lgbtqia-only) / 1 multi (immigrant+women)", m)
	}
	if got := tally(r, "lgbtqia").Works; got != 1 {
		t.Errorf("lgbtqia works = %d, want 1 (two subjects, one work)", got)
	}
	if got := tally(r, "immigrant-diaspora").Works; got != 1 {
		t.Errorf("immigrant works = %d, want 1", got)
	}
	if got := tally(r, "women-gender").Works; got != 1 {
		t.Errorf("women-gender works = %d, want 1", got)
	}
	// Shares are against the right denominators.
	lg := tally(r, "lgbtqia")
	if !approx(lg.ShareCovered, 1.0/3.0) {
		t.Errorf("lgbtqia ShareCovered = %v, want 1/3 (of 3 covered)", lg.ShareCovered)
	}
	if !approx(lg.ShareTotal, 1.0/4.0) {
		t.Errorf("lgbtqia ShareTotal = %v, want 1/4 (of 4 total)", lg.ShareTotal)
	}
}

// TestAuditorURIAndLabelBothCount checks that a URI-only subject counts toward
// coverage and category via an override URI, alongside label matching.
func TestAuditorURIAndLabelBothCount(t *testing.T) {
	cw := Default()
	a := NewAuditor(cw)
	// A URI-only subject that maps nowhere: covered, no category.
	a.Add([]SubjectRef{sub("http://example.org/x")})
	r := a.Report()
	if r.CoveredWorks != 1 {
		t.Errorf("a URI-only subject should count as covered: %d", r.CoveredWorks)
	}
	for _, c := range r.Categories {
		if c.Works != 0 {
			t.Errorf("unexpected category hit %s=%d", c.ID, c.Works)
		}
	}
}

// TestAuditorLanguageCoverage checks the per-language subject-label tally: a
// work counts once per language however many of its subjects carry that
// language, a work's languages union across its matching subjects, and a work
// matched only through label-less subjects lands in no language column while
// still counting toward Works.
func TestAuditorLanguageCoverage(t *testing.T) {
	a := NewAuditor(Default())

	// 1: lgbtqia via two en+es subjects -> en:1, es:1 (counted once each).
	a.Add([]SubjectRef{
		{Labels: []string{"Lesbian fiction"}, Langs: []string{"en", "es"}},
		{Labels: []string{"Gay men"}, Langs: []string{"en", "es"}},
	})
	// 2: lgbtqia via an English-only subject -> en only.
	a.Add([]SubjectRef{{Labels: []string{"Queer theory"}, Langs: []string{"en"}}})
	// 3: lgbtqia through an en+fr and an English-only subject -> union en, fr.
	a.Add([]SubjectRef{
		{Labels: []string{"Transgender people"}, Langs: []string{"en", "fr"}},
		{Labels: []string{"Gay men"}, Langs: []string{"en"}},
	})

	r := a.Report()
	lg := tally(r, "lgbtqia")
	if lg.Works != 3 {
		t.Fatalf("lgbtqia works = %d, want 3", lg.Works)
	}
	if lg.LabelLangWorks["en"] != 3 {
		t.Errorf("lgbtqia en = %d, want 3 (all works carry an en label)", lg.LabelLangWorks["en"])
	}
	if lg.LabelLangWorks["es"] != 1 {
		t.Errorf("lgbtqia es = %d, want 1 (work 1)", lg.LabelLangWorks["es"])
	}
	if lg.LabelLangWorks["fr"] != 1 {
		t.Errorf("lgbtqia fr = %d, want 1 (work 3)", lg.LabelLangWorks["fr"])
	}
	// A language a term never carried is absent, not a zero entry.
	if _, ok := lg.LabelLangWorks["de"]; ok {
		t.Errorf("lgbtqia should carry no de entry, got %d", lg.LabelLangWorks["de"])
	}
}

// TestAuditorBookLanguage checks the per-category BOOK-language tally (task
// 495): a work counts under its single resolved book language, a multilingual
// work lands in BookLangMulti not any per-language column, a work with no book
// language contributes to neither, and the axis is distinct from the
// subject-heading LabelLangWorks reachability.
func TestAuditorBookLanguage(t *testing.T) {
	a := NewAuditor(Default())
	// 1: lgbtqia, English book. Its subject carries en+es labels, so the label
	// axis says es-reachable while the book axis says English -- the exact
	// conflation 495 separates.
	a.AddWeighted([]SubjectRef{{Labels: []string{"Gay men"}, Langs: []string{"en", "es"}}}, 0, "eng")
	// 2: lgbtqia, Spanish book, subject reachable in en+es.
	a.AddWeighted([]SubjectRef{{Labels: []string{"Lesbian fiction"}, Langs: []string{"en", "es"}}}, 0, "spa")
	// 3: lgbtqia, multilingual book (two languages) -> BookLangMulti.
	a.AddWeighted([]SubjectRef{{Labels: []string{"Queer theory"}, Langs: []string{"en"}}}, 0, "eng", "spa")
	// 4: lgbtqia, no book language declared -> neither column.
	a.AddWeighted([]SubjectRef{{Labels: []string{"Transgender people"}, Langs: []string{"en"}}}, 0)

	lg := tally(a.Report(), "lgbtqia")
	if lg.Works != 4 {
		t.Fatalf("lgbtqia works = %d, want 4", lg.Works)
	}
	if lg.BookLangWorks["eng"] != 1 {
		t.Errorf("lgbtqia eng books = %d, want 1 (work 1 only; the multilingual work is excluded)", lg.BookLangWorks["eng"])
	}
	if lg.BookLangWorks["spa"] != 1 {
		t.Errorf("lgbtqia spa books = %d, want 1 (work 2 only)", lg.BookLangWorks["spa"])
	}
	if lg.BookLangMulti != 1 {
		t.Errorf("lgbtqia multilingual books = %d, want 1 (work 3)", lg.BookLangMulti)
	}
	// The two axes are computed independently: es-heading reachability counts
	// every work whose subject carries an es label (works 1 and 2), while es
	// books counts only the Spanish book (work 2) -- the inflation 495 separates.
	if lg.LabelLangWorks["es"] != 2 {
		t.Errorf("lgbtqia es labels = %d, want 2 (works 1,2 carry es-labeled subjects)", lg.LabelLangWorks["es"])
	}
	if lg.BookLangWorks["spa"] != 1 {
		t.Errorf("lgbtqia es books = %d, want 1 -- must not inflate to the es-label count", lg.BookLangWorks["spa"])
	}
}

// TestAuditorLanglessSubject checks that a subject with no configured-language
// labels (an uncontrolled heading) counts toward Works but no language column.
func TestAuditorLanglessSubject(t *testing.T) {
	a := NewAuditor(Default())
	a.Add([]SubjectRef{{Labels: []string{"Gay men"}}}) // no Langs
	lg := tally(a.Report(), "lgbtqia")
	if lg.Works != 1 {
		t.Fatalf("lgbtqia works = %d, want 1", lg.Works)
	}
	if len(lg.LabelLangWorks) != 0 {
		t.Errorf("langless subject should populate no language column, got %v", lg.LabelLangWorks)
	}
}

// TestAuditorWeighted checks the per-work weight tally: a category's Weight is
// the sum of its works' weights (double-counting across categories like Works
// does), TotalWeight sums every work's weight including uncategorized ones, and
// an unweighted Add contributes 0.
func TestAuditorWeighted(t *testing.T) {
	a := NewAuditor(Default())
	a.AddWeighted([]SubjectRef{sub("", "Lesbian fiction")}, 5) // lgbtqia, weight 5
	a.AddWeighted([]SubjectRef{sub("", "Gay men")}, 3)         // lgbtqia, weight 3
	a.AddWeighted([]SubjectRef{sub("", "Cooking")}, 2)         // no category, weight 2
	a.Add([]SubjectRef{sub("", "Queer theory")})               // lgbtqia, weight 0

	r := a.Report()
	if r.TotalWeight != 10 {
		t.Errorf("TotalWeight = %d, want 10 (5+3+2, incl. uncategorized)", r.TotalWeight)
	}
	if lg := tally(r, "lgbtqia").Weight; lg != 8 {
		t.Errorf("lgbtqia weight = %d, want 8 (5+3+0)", lg)
	}
	// A category with no weighted works reports zero weight.
	if w := tally(r, "immigrant-diaspora").Weight; w != 0 {
		t.Errorf("immigrant-diaspora weight = %d, want 0", w)
	}
}

// TestAuditorUnweightedHasNoWeight checks that a purely unweighted audit (the
// CLI path) reports no weight, so the field stays absent for callers that never
// supply one.
func TestAuditorUnweightedHasNoWeight(t *testing.T) {
	a := NewAuditor(Default())
	a.Add([]SubjectRef{sub("", "Gay men")})
	r := a.Report()
	if r.TotalWeight != 0 || tally(r, "lgbtqia").Weight != 0 {
		t.Errorf("unweighted audit should carry no weight, got total %d / lgbtqia %d", r.TotalWeight, tally(r, "lgbtqia").Weight)
	}
}

// TestAuditorEmptyCorpus checks the divide-by-zero guards: an empty corpus reports
// zero coverage and zero shares, not NaN.
func TestAuditorEmptyCorpus(t *testing.T) {
	r := NewAuditor(Default()).Report()
	if r.TotalWorks != 0 || r.CoveredWorks != 0 || r.Coverage != 0 {
		t.Errorf("empty corpus totals wrong: %+v", r)
	}
	for _, c := range r.Categories {
		if c.ShareCovered != 0 || c.ShareTotal != 0 {
			t.Errorf("empty corpus share should be 0, got %+v", c)
		}
	}
	if len(r.Categories) == 0 {
		t.Error("report should still list the categories (all zero)")
	}
}
