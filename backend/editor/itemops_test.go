package editor

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/freeeve/libcat/bibframe"
)

// itemGrain takes one real grain and shelves three copies on its first
// instance, so the item ops run against the same shapes the item panel writes.
func itemGrain(t *testing.T) (m *Mapper, workID string, grain []byte, instID string) {
	t.Helper()
	m = newMapper(t)
	for id, g := range realGrains(t) {
		workID, grain = id, g
		break
	}
	doc, err := m.ToDoc(grain, workID)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Instances) == 0 {
		t.Fatal("grain has no instance")
	}
	instID = doc.Instances[0].ID
	grain, err = bibframe.SetItems(grain, instID, []bibframe.Item{
		{CallNumber: "FIC UNG", Location: "Stacks", Barcode: "31234"},
		{CallNumber: "FIC UNG", Location: "Stacks", Barcode: "31235"},
		{CallNumber: "FIC UNG", Location: "Reference", Barcode: "31236"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return m, workID, grain, instID
}

func setLocation(v string, where *string) Op {
	return Op{Resource: ResourceItems, Path: "location", Action: "set", Values: []OpValue{{V: v}}, Where: where}
}

// The point of item ops: a relocation runs through ApplyOps, so a batch,
// a macro, and the single-record editor all reach holdings the same way.
func TestApplyOpsRelocatesItems(t *testing.T) {
	m, workID, grain, instID := itemGrain(t)
	stacks := "Stacks"
	out, err := ApplyOps(m, grain, workID, []Op{setLocation("Annex", &stacks)}, nil)
	if err != nil {
		t.Fatalf("ApplyOps: %v", err)
	}
	items, err := bibframe.ItemsOf(out, instID)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Annex", "Annex", "Reference"}
	if len(items) != 3 {
		t.Fatalf("items = %+v", items)
	}
	for i, it := range items {
		if it.Location != want[i] {
			t.Fatalf("item %d location = %q, want %q", i, it.Location, want[i])
		}
		// The guard moves the shelf, not the copy: barcodes and call numbers
		// survive an edit that only names location.
		if it.Barcode == "" || it.CallNumber != "FIC UNG" {
			t.Fatalf("item %d lost a sibling field: %+v", i, it)
		}
	}
	// The diff a dry run shows is exactly the two moved copies.
	diff := DiffLines(grain, out)
	if len(diff.Added) != 2 || len(diff.Removed) != 2 {
		t.Fatalf("diff = +%d/-%d, want +2/-2", len(diff.Added), len(diff.Removed))
	}
}

func TestApplyOpsClearsAnItemField(t *testing.T) {
	m, workID, grain, instID := itemGrain(t)
	out, err := ApplyOps(m, grain, workID, []Op{{Resource: ResourceItems, Path: "callNumber", Action: "clear"}}, nil)
	if err != nil {
		t.Fatalf("ApplyOps: %v", err)
	}
	items, err := bibframe.ItemsOf(out, instID)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range items {
		if it.CallNumber != "" {
			t.Fatalf("item %s kept callNumber %q", it.ID, it.CallNumber)
		}
		if it.Barcode == "" {
			t.Fatalf("item %s lost its barcode", it.ID)
		}
	}
}

// Work ops and item ops compose in one list: they build one patch, applied
// once, so a macro can retag a selection and reshelve it together.
func TestApplyOpsMixesWorkAndItemOps(t *testing.T) {
	m, workID, grain, instID := itemGrain(t)
	out, err := ApplyOps(m, grain, workID, []Op{
		{Resource: "work", Path: "tags", Action: "add", Value: &OpValue{V: "weeded"}},
		setLocation("Annex", nil),
	}, nil)
	if err != nil {
		t.Fatalf("ApplyOps: %v", err)
	}
	doc, err := m.ToDoc(out, workID)
	if err != nil {
		t.Fatal(err)
	}
	var tagged bool
	for _, v := range doc.Work.Fields["tags"] {
		if v.V == "weeded" {
			tagged = true
		}
	}
	if !tagged {
		t.Fatalf("work tags = %+v, want a weeded tag", doc.Work.Fields["tags"])
	}
	items, err := bibframe.ItemsOf(out, instID)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range items {
		if it.Location != "Annex" {
			t.Fatalf("item %s location = %q, want Annex", it.ID, it.Location)
		}
	}
}

func TestApplyOpsRefusesBadItemOps(t *testing.T) {
	m, workID, grain, _ := itemGrain(t)
	cases := []struct {
		name string
		ops  []Op
		want string
	}{
		{"add is not an item action", []Op{{Resource: ResourceItems, Path: "location", Action: "add", Value: &OpValue{V: "Annex"}}}, "use set or clear"},
		{"remove is not an item action", []Op{{Resource: ResourceItems, Path: "location", Action: "remove", Value: &OpValue{V: "Stacks"}}}, "use set or clear"},
		{"set takes one value", []Op{{Resource: ResourceItems, Path: "location", Action: "set", Values: []OpValue{{V: "a"}, {V: "b"}}}}, "exactly one value"},
		{"set needs a value", []Op{{Resource: ResourceItems, Path: "location", Action: "set", Values: []OpValue{{V: ""}}}}, "use clear"},
		{"item fields are text", []Op{{Resource: ResourceItems, Path: "location", Action: "set", Values: []OpValue{{V: "http://x", IRI: true}}}}, "not IRIs"},
		{"clear takes no value", []Op{{Resource: ResourceItems, Path: "note", Action: "clear", Values: []OpValue{{V: "x"}}}}, "takes no value"},
		{"barcode is unreachable", []Op{setBarcode("31234")}, "no such item field"},
		{"unknown field", []Op{{Resource: ResourceItems, Path: "shelf", Action: "clear"}}, "no such item field"},
		{"one field twice", []Op{setLocation("Annex", nil), setLocation("Stacks", nil)}, "edited twice"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ApplyOps(m, grain, workID, tc.ops, nil)
			if err == nil {
				t.Fatalf("accepted %s", tc.name)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}

func setBarcode(v string) Op {
	return Op{Resource: ResourceItems, Path: "barcode", Action: "set", Values: []OpValue{{V: v}}}
}

// TestItemFieldsMatchUI pins the batch screen's item-field picker to the Go
// table it mirrors. If the two drift, a cataloger picks a field the server
// refuses -- or, worse, the picker offers barcode.
func TestItemFieldsMatchUI(t *testing.T) {
	const uiPath = "../ui/src/lib/itemops.ts"
	src, err := os.ReadFile(uiPath)
	if err != nil {
		t.Fatal(err)
	}
	block := regexp.MustCompile(`(?s)export const ITEM_FIELDS[^\[]*\[(.*?)\n\];`).FindSubmatch(src)
	if block == nil {
		t.Fatalf("%s: ITEM_FIELDS table not found; did it move?", uiPath)
	}
	var ui []string
	for _, m := range regexp.MustCompile(`path:\s*"([^"]+)"`).FindAllSubmatch(block[1], -1) {
		ui = append(ui, string(m[1]))
	}
	sort.Strings(ui)
	got := bibframe.ItemFieldNames()
	if strings.Join(ui, ",") != strings.Join(got, ",") {
		t.Errorf("item fields disagree:\n  %s: %v\n  bibframe.ItemFieldNames: %v\nupdate both tables together", uiPath, ui, got)
	}
	for _, name := range ui {
		if name == "barcode" {
			t.Error("the batch picker offers barcode; a barcode names one copy")
		}
	}
}
