package project

import (
	"context"
	"os"
	"path/filepath"
	"reflect"

	"testing"

	"github.com/freeeve/libcat/ingest"
	"github.com/freeeve/libcat/ingest/marc"
	codex "github.com/freeeve/libcodex"
)

// seriesProvider feeds pre-built records so a test corpus needs no fixture file.
type seriesProvider struct{ recs []ingest.Record }

func (seriesProvider) Name() string                                       { return "marc" }
func (seriesProvider) Role() ingest.Role                                  { return ingest.RoleIngest }
func (p seriesProvider) Records(context.Context) ([]ingest.Record, error) { return p.recs, nil }

// projectMARC ingests records through libcodex and projects the resulting graph.
//
// Deliberately end-to-end rather than a hand-written nquads fixture. The flat
// Instance literals this code used to read were pinned by exactly such a fixture,
// so when libcodex v0.25.0 moved series onto a bf:relation on the Work, the
// projector returned empty and every test stayed green: the fixture agreed with
// the reader, and neither agreed with libcodex.
func projectMARC(t *testing.T, recs ...*codex.Record) *Catalog {
	t.Helper()
	dir := t.TempDir()
	if _, err := ingest.Run(seriesProvider{recs: marc.FromCodexRecords(recs)}, dir); err != nil {
		t.Fatal(err)
	}
	nq, err := os.ReadFile(filepath.Join(dir, "catalog.nq"))
	if err != nil {
		t.Fatal(err)
	}
	cat, err := Project(nq, "marc")
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	return cat
}

// seriesRecord is a monograph carrying the given 490 fields.
func seriesRecord(control, title string, f490 ...codex.Field) *codex.Record {
	r := codex.NewRecord()
	r.SetLeader(codex.Leader([]byte("00000nam a2200000 a 4500")))
	r.AddField(codex.NewControlField("001", control))
	r.AddField(codex.NewDataField("245", '1', '0', codex.NewSubfield('a', title)))
	for _, f := range f490 {
		r.AddField(f)
	}
	return r
}

// . Two 490s, each with its own $v. The flat shape paired a statement to
// an enumeration by list position, and an RDF graph is a set -- so this record
// used to yield one enumeration for the whole Instance, and the projector took
// whichever came first. Each enumeration now belongs to its own relation.
func TestTwo490sEachKeepTheirOwnEnumeration(t *testing.T) {
	cat := projectMARC(t, seriesRecord("c1", "A Book",
		codex.NewDataField("490", '1', ' ',
			codex.NewSubfield('a', "Firebrand fiction"),
			codex.NewSubfield('v', "bk. 2"),
			codex.NewSubfield('x', "0075-2118")),
		codex.NewDataField("490", '0', ' ',
			codex.NewSubfield('a', "Second series"),
			codex.NewSubfield('v', "v. 7"))))

	if len(cat.Works) != 1 {
		t.Fatalf("want 1 work, got %d", len(cat.Works))
	}
	want := []Series{
		// Traced: 490 ind1=1 gave the first series an added entry. ISSN is 490$x,
		// which the flat mapping dropped on the floor.
		{Title: "Firebrand fiction", Enumeration: "bk. 2", ISSN: "0075-2118", Traced: true},
		{Title: "Second series", Enumeration: "v. 7"},
	}
	if !reflect.DeepEqual(cat.Works[0].Series, want) {
		t.Fatalf("series = %+v\nwant %+v", cat.Works[0].Series, want)
	}
}

// The control for the test above, and the one that makes it mean anything: a
// record with no 490 projects no series. Without it, a `series` field that always
// returned the same two entries would satisfy the assertion above.
func TestARecordWithNo490ProjectsNoSeries(t *testing.T) {
	cat := projectMARC(t, seriesRecord("c1", "A Book"))
	if len(cat.Works[0].Series) != 0 {
		t.Fatalf("series = %+v, want none", cat.Works[0].Series)
	}
}

// A 490 with only $v -- an enumeration with nothing to enumerate -- is not a
// series anyone can render. "bk. 2" of what?
func TestASeriesWithNoStatementIsDropped(t *testing.T) {
	cat := projectMARC(t, seriesRecord("c1", "A Book",
		codex.NewDataField("490", '0', ' ', codex.NewSubfield('v', "bk. 2"))))
	if len(cat.Works[0].Series) != 0 {
		t.Fatalf("series = %+v, want none", cat.Works[0].Series)
	}
}

// bf:relation is not a series predicate. It carries any related resource -- a
// translation, a sequel, an earlier edition -- and only the bf:relationship IRI
// says which one is a series. A reader that walked bf:relation without checking
// would project every linking entry as a series statement.
//
// Written as an nquads fixture on purpose: it drives Project directly on a graph
// carrying an arbitrary non-series relationship IRI, covering the paths libcodex
// does not emit from MARC -- editorial writes and nquads ingest -- and relations
// it may map in future. The original reason ("a MARC record cannot produce a
// competing relation") held only for 765/830; 780/785 do emit one, so
// TestANonSeriesRelationFromAReal780RecordIsNotProjectedAsASeries below now pins
// the same guard on a real record too.
func TestANonSeriesRelationIsNotProjectedAsASeries(t *testing.T) {
	const bf = "http://id.loc.gov/ontologies/bibframe/"
	nq := `<#waaWork> <http://www.w3.org/1999/02/22-rdf-syntax-ns#type> <` + bf + `Work> <feed:marc> .
<#waaWork> <` + bf + `title> _:wt <feed:marc> .
_:wt <` + bf + `mainTitle> "A Book" <feed:marc> .
<#waaWork> <` + bf + `relation> _:relSeries <feed:marc> .
_:relSeries <` + bf + `relationship> <http://id.loc.gov/vocabulary/relationship/series> <feed:marc> .
_:relSeries <` + bf + `associatedResource> _:series <feed:marc> .
_:series <` + bf + `title> _:st <feed:marc> .
_:st <` + bf + `mainTitle> "Real Series" <feed:marc> .
<#waaWork> <` + bf + `relation> _:relTrans <feed:marc> .
_:relTrans <` + bf + `relationship> <http://id.loc.gov/vocabulary/relationship/translationof> <feed:marc> .
_:relTrans <` + bf + `associatedResource> _:orig <feed:marc> .
_:orig <` + bf + `title> _:ot <feed:marc> .
_:ot <` + bf + `mainTitle> "Original Title" <feed:marc> .
`
	cat, err := Project([]byte(nq), "marc")
	if err != nil {
		t.Fatal(err)
	}
	got := cat.Works[0].Series
	// The positive half: the series relation still projects. Without it, a reader
	// that dropped every relation would satisfy the negative half below.
	if len(got) != 1 || got[0].Title != "Real Series" {
		t.Fatalf("series = %+v, want exactly the series relation", got)
	}
	for _, s := range got {
		if s.Title == "Original Title" {
			t.Error("a translationOf relation projected as a series")
		}
	}
}

// The real-record companion to the fixture test above (answering
// libcodex 112): a record carrying a 490 and a 780 produces exactly the competing
// pair the guard must tell apart. libcodex maps 780 (preceding title) to a
// bf:relation whose relationship is *not* series (ind2=0 -> `continues`), while
// the 490 maps to a series relation. The projector must take only the 490's
// series and never the 780's preceding title. This exercises the real
// FromRecord -> Project pipeline, so it is immune to the fixture-agrees-with-the-
// reader failure that let the flat-shape bug ship green; deleting the
// relationship check would surface the 780's $t "Old Title" as a spurious series,
// the mis-read libcodex 112 warned about.
func TestANonSeriesRelationFromAReal780RecordIsNotProjectedAsASeries(t *testing.T) {
	rec := seriesRecord("c1", "A Book",
		codex.NewDataField("490", '0', ' ', codex.NewSubfield('a', "Firebrand fiction")))
	rec.AddField(codex.NewDataField("780", '0', '0', codex.NewSubfield('t', "Old Title")))
	cat := projectMARC(t, rec)

	got := cat.Works[0].Series
	if len(got) != 1 || got[0].Title != "Firebrand fiction" {
		t.Fatalf("series = %+v, want exactly the 490 series", got)
	}
	for _, s := range got {
		if s.Title == "Old Title" {
			t.Error("the 780 preceding-title relation projected as a series")
		}
	}
}

// A bf:Series may carry several identifiers; only the bf:Issn is its ISSN. A
// MARC record cannot exercise this -- libcodex emits exactly one identifier on
// the series node, so dropping the type check leaves the whole suite green, which
// is what happened when it was first written. This drives Project on a graph that
// carries two.
func TestOnlyTheIssnIdentifierIsTakenAsTheSeriesISSN(t *testing.T) {
	const bf = "http://id.loc.gov/ontologies/bibframe/"
	const rdfValue = "http://www.w3.org/1999/02/22-rdf-syntax-ns#value"
	const rdfType = "http://www.w3.org/1999/02/22-rdf-syntax-ns#type"
	nq := `<#waaWork> <` + rdfType + `> <` + bf + `Work> <feed:marc> .
<#waaWork> <` + bf + `title> _:wt <feed:marc> .
_:wt <` + bf + `mainTitle> "A Book" <feed:marc> .
<#waaWork> <` + bf + `relation> _:rel <feed:marc> .
_:rel <` + bf + `relationship> <http://id.loc.gov/vocabulary/relationship/series> <feed:marc> .
_:rel <` + bf + `associatedResource> _:series <feed:marc> .
_:series <` + bf + `title> _:st <feed:marc> .
_:st <` + bf + `mainTitle> "Firebrand fiction" <feed:marc> .
_:series <` + bf + `identifiedBy> _:lccn <feed:marc> .
_:lccn <` + rdfType + `> <` + bf + `Lccn> <feed:marc> .
_:lccn <` + rdfValue + `> "sn 85-1234" <feed:marc> .
_:series <` + bf + `identifiedBy> _:issn <feed:marc> .
_:issn <` + rdfType + `> <` + bf + `Issn> <feed:marc> .
_:issn <` + rdfValue + `> "0075-2118" <feed:marc> .
`
	cat, err := Project([]byte(nq), "marc")
	if err != nil {
		t.Fatal(err)
	}
	got := cat.Works[0].Series
	if len(got) != 1 {
		t.Fatalf("series = %+v", got)
	}
	// Present: the ISSN is read. Absent: the LCCN is not mistaken for one. Both
	// halves, or a reader that took whichever identifier came last would pass.
	if got[0].ISSN != "0075-2118" {
		t.Errorf("issn = %q, want the bf:Issn value", got[0].ISSN)
	}
	if got[0].ISSN == "sn 85-1234" {
		t.Error("the LCCN was projected as the series ISSN")
	}
}

// The enumeration hangs off the relation, not off the series. Two Works in one
// series are numbered differently, and the series node may be shared.
func TestOneSeriesNumbersTwoWorksDifferently(t *testing.T) {
	cat := projectMARC(t,
		seriesRecord("c1", "First", codex.NewDataField("490", '0', ' ',
			codex.NewSubfield('a', "Firebrand fiction"), codex.NewSubfield('v', "bk. 1"))),
		seriesRecord("c2", "Second", codex.NewDataField("490", '0', ' ',
			codex.NewSubfield('a', "Firebrand fiction"), codex.NewSubfield('v', "bk. 2"))))

	byTitle := map[string]Work{}
	for _, w := range cat.Works {
		byTitle[w.Title] = w
	}
	if len(byTitle) != 2 {
		t.Fatalf("want 2 works, got %d", len(byTitle))
	}
	for title, wantEnum := range map[string]string{"First": "bk. 1", "Second": "bk. 2"} {
		got := byTitle[title].Series
		if len(got) != 1 || got[0].Enumeration != wantEnum || got[0].Title != "Firebrand fiction" {
			t.Errorf("%q series = %+v, want %q of Firebrand fiction", title, got, wantEnum)
		}
	}
}

// The scorer sees series titles, not enumerations: "bk. 2" and "bk. 7" of one
// series are neighbours, and two unrelated series sharing "v. 1" are not.
func TestSimilarWorkTakesSeriesTitlesNotEnumerations(t *testing.T) {
	w := Work{ID: "w1", Series: []Series{{Title: "Firebrand fiction", Enumeration: "bk. 2"}}}
	got := w.SimilarWork()
	if !reflect.DeepEqual(got.Series, []string{"Firebrand fiction"}) {
		t.Fatalf("scorer series = %v", got.Series)
	}
}

// A cataloged record typically carries the PAIR: the 490 transcription and
// its 830 controlled form. libcodex v0.33.0 emits both as series relations
// (the 830's resource additionally typed bf:Hub), and one membership must
// not render twice: the pair merges by title, traced, with the enumeration
// from the 490 and the ISSN from whichever side carries it (task 436).
func TestA490AndIts830PairProjectOneMergedSeries(t *testing.T) {
	cat := projectMARC(t, seriesRecord("c1", "A Book",
		codex.NewDataField("490", '1', ' ',
			codex.NewSubfield('a', "Firebrand fiction ;"),
			codex.NewSubfield('v', "bk. 2")),
		codex.NewDataField("830", ' ', '0',
			codex.NewSubfield('a', "Firebrand fiction"),
			codex.NewSubfield('x', "0075-2118"))))

	want := []Series{
		{Title: "Firebrand fiction", Enumeration: "bk. 2", ISSN: "0075-2118", Traced: true},
	}
	if !reflect.DeepEqual(cat.Works[0].Series, want) {
		t.Fatalf("series = %+v\nwant %+v", cat.Works[0].Series, want)
	}
}

// An 830 with no 490 at all (some vendors catalog only the controlled form)
// still projects, and projects traced: the controlled added entry IS the
// trace, even with no mstatus/tr in sight (task 436).
func TestAnAloneControlled830ProjectsATracedSeries(t *testing.T) {
	cat := projectMARC(t, seriesRecord("c1", "A Book",
		codex.NewDataField("830", ' ', '0',
			codex.NewSubfield('a', "Herculine"),
			codex.NewSubfield('v', "v. 3"))))

	want := []Series{{Title: "Herculine", Enumeration: "v. 3", Traced: true}}
	if !reflect.DeepEqual(cat.Works[0].Series, want) {
		t.Fatalf("series = %+v\nwant %+v", cat.Works[0].Series, want)
	}
}

// A 762 subseries entry names a series membership through the subseries
// relationship IRI; it must project like the rest (task 436).
func TestA762SubseriesEntryProjectsAsASeries(t *testing.T) {
	cat := projectMARC(t, seriesRecord("c1", "A Book",
		codex.NewDataField("762", '0', ' ',
			codex.NewSubfield('t', "Companion volumes"))))

	if len(cat.Works[0].Series) != 1 || cat.Works[0].Series[0].Title != "Companion volumes" {
		t.Fatalf("series = %+v, want the 762 subseries", cat.Works[0].Series)
	}
	if !cat.Works[0].Series[0].Traced {
		t.Fatalf("a controlled linking entry is the trace; got %+v", cat.Works[0].Series[0])
	}
}

// An 800 heading is name + title; the series title is its $t (bf:title on
// the Hub), not the personal name, which rides as a contribution (task 436).
func TestAn800PersonalNameSeriesProjectsItsTitle(t *testing.T) {
	cat := projectMARC(t, seriesRecord("c1", "A Book",
		codex.NewDataField("800", '1', ' ',
			codex.NewSubfield('a', "Le Guin, Ursula K."),
			codex.NewSubfield('t', "Earthsea cycle"),
			codex.NewSubfield('v', "bk. 4"))))

	want := []Series{{Title: "Earthsea cycle", Enumeration: "bk. 4", Traced: true}}
	if !reflect.DeepEqual(cat.Works[0].Series, want) {
		t.Fatalf("series = %+v\nwant %+v", cat.Works[0].Series, want)
	}
}

// Two DIFFERENT series -- a 490 and an unrelated 830 -- must stay two
// entries; the pair-merge keys on the title, not on the shapes (task 436).
func TestAnUnrelated830DoesNotMergeWithA490(t *testing.T) {
	cat := projectMARC(t, seriesRecord("c1", "A Book",
		codex.NewDataField("490", '0', ' ', codex.NewSubfield('a', "Second series")),
		codex.NewDataField("830", ' ', '0', codex.NewSubfield('a', "Herculine"))))

	if len(cat.Works[0].Series) != 2 {
		t.Fatalf("series = %+v, want two distinct entries", cat.Works[0].Series)
	}
}
