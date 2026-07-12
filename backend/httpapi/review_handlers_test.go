package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/freeeve/libcat/backend/auth"
	"github.com/freeeve/libcat/backend/store"
	"github.com/freeeve/libcat/backend/suggest"
	"github.com/freeeve/libcat/backend/vocab"
	"github.com/freeeve/libcat/storage/blob"
)

// staffVerifier maps fixed tokens to staff identities.
type staffVerifier map[string]auth.Identity

func (v staffVerifier) Verify(ctx context.Context, raw string) (auth.Identity, error) {
	if id, ok := v[raw]; ok {
		return id, nil
	}
	return auth.Identity{}, auth.ErrUnauthorized
}

func newModerationAPI(t *testing.T) (http.Handler, *suggest.Service) {
	t.Helper()
	data, err := os.ReadFile("../vocab/testdata/authorities.nq")
	if err != nil {
		t.Fatal(err)
	}
	bs := blob.NewMem()
	_, _ = bs.Put(t.Context(), "a/x.nq", data, blob.PutOptions{})
	ix, err := vocab.Load(t.Context(), bs, "a/", nil)
	if err != nil {
		t.Fatal(err)
	}
	svc := suggest.New(store.NewMem(), ix, suggest.Caps{})
	// Open the patron intake; default is off.
	if _, err := svc.PutPolicy(t.Context(), suggest.Policy{Enabled: true, FreeText: suggest.FreeTextAny}); err != nil {
		t.Fatal(err)
	}
	verifier := staffVerifier{
		"mod-token": {Email: "mod@example.org", Roles: []auth.Role{auth.RoleModerator}},
		"lib-token": {Email: "lib@example.org", Roles: []auth.Role{auth.RoleLibrarian}},
	}
	abuse, _ := suggest.NewAbuse([]byte("0123456789abcdef0123456789abcdef"))
	return New(Deps{Suggest: svc, Abuse: abuse, Vocab: ix, Verifier: verifier}), svc
}

func seed(t *testing.T, svc *suggest.Service, workID string) {
	t.Helper()
	_, err := svc.Submit(t.Context(), suggest.SubmitInput{
		WorkID: workID,
		Term:   vocab.TermRef{Scheme: "homosaurus", ID: transURI},
		Type:   suggest.TypeAdd, SupporterHash: "seed-hash", WorkTitle: "A Book",
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestModerationFlow(t *testing.T) {
	h, svc := newModerationAPI(t)
	seed(t, svc, "wabc123def456")

	// Queue requires staff.
	if rec := doJSON(t, h, http.MethodGet, "/v1/queue", "", nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous queue: %d", rec.Code)
	}
	rec := doJSON(t, h, http.MethodGet, "/v1/queue", "mod-token", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("queue: %d %s", rec.Code, rec.Body)
	}
	var page suggest.QueuePage
	_ = json.Unmarshal(rec.Body.Bytes(), &page)
	if len(page.Items) != 1 {
		t.Fatalf("queue = %+v", page)
	}

	// Moderator approves but cannot publish.
	review := map[string]any{
		"decisions": []map[string]any{{
			"workId": "wabc123def456",
			"term":   map[string]string{"scheme": "homosaurus", "id": transURI},
			"type":   "ADD", "approve": true,
		}},
		"publish": true,
	}
	rec = doJSON(t, h, http.MethodPost, "/v1/review", "mod-token", review)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("moderator publish: %d, want 403", rec.Code)
	}
	review["publish"] = false
	rec = doJSON(t, h, http.MethodPost, "/v1/review", "mod-token", review)
	if rec.Code != http.StatusOK {
		t.Fatalf("moderator review: %d %s", rec.Code, rec.Body)
	}

	// Librarian may set publish (queued until the publisher lands).
	seed(t, svc, "wzzz999zzz999")
	review["decisions"].([]map[string]any)[0]["workId"] = "wzzz999zzz999"
	review["publish"] = true
	rec = doJSON(t, h, http.MethodPost, "/v1/review", "lib-token", review)
	if rec.Code != http.StatusOK {
		t.Fatalf("librarian review: %d %s", rec.Code, rec.Body)
	}
	var resp map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["approvedPending"].(float64) < 2 {
		t.Fatalf("resp = %v", resp)
	}

	// Audit: librarian only, month-validated.
	if rec := doJSON(t, h, http.MethodGet, "/v1/audit?month=2026-07", "mod-token", nil); rec.Code != http.StatusForbidden {
		t.Fatalf("moderator audit: %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodGet, "/v1/audit?month=nope", "lib-token", nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad month: %d", rec.Code)
	}
	month := page.Items[0].CreatedAt.UTC().Format("2006-01")
	rec = doJSON(t, h, http.MethodGet, "/v1/audit?month="+month, "lib-token", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("audit: %d", rec.Code)
	}
	var audit struct {
		Entries []suggest.AuditEntry `json:"entries"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &audit)
	if len(audit.Entries) != 2 {
		t.Fatalf("audit entries = %+v", audit.Entries)
	}
}

// TestMonthDefaultsToCurrentUTC covers on the month-keyed staff
// reports an absent month means "this month", while a malformed one still
// refuses. Absent and wrong are different answers, not the same 400.
func TestMonthDefaultsToCurrentUTC(t *testing.T) {
	h, _ := newModerationAPI(t)
	now := time.Now().UTC().Format("2006-01")

	for _, path := range []string{"/v1/stats", "/v1/audit"} {
		rec := doJSON(t, h, http.MethodGet, path, "lib-token", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s with no month = %d %s", path, rec.Code, rec.Body)
		}
		var body struct {
			Month string `json:"month"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body.Month != now {
			t.Errorf("GET %s defaulted to month %q, want the current UTC month %q", path, body.Month, now)
		}
		// Wrong is still wrong, and the message shows the shape.
		rec = doJSON(t, h, http.MethodGet, path+"?month=nope", "lib-token", nil)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("GET %s?month=nope = %d, want 400", path, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "month=2026-07") {
			t.Errorf("GET %s?month=nope error names no example: %s", path, rec.Body)
		}
		// An empty value reads as absent -- ?month= is what a form with a
		// cleared field sends, and it means the same thing as omitting it.
		if rec := doJSON(t, h, http.MethodGet, path+"?month=", "lib-token", nil); rec.Code != http.StatusOK {
			t.Errorf("GET %s?month= (empty) = %d, want the default", path, rec.Code)
		}
	}
}

func TestManualAndFolkGovernanceRoutes(t *testing.T) {
	h, svc := newModerationAPI(t)

	// Manual term: librarian only.
	manual := map[string]any{
		"action": "manual", "workId": "wabc123def456",
		"term": map[string]string{"scheme": "homosaurus", "id": transURI},
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/terms", "mod-token", manual); rec.Code != http.StatusForbidden {
		t.Fatalf("moderator manual term: %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/terms", "lib-token", manual); rec.Code != http.StatusCreated {
		t.Fatalf("manual term: %d", rec.Code)
	}

	// Folk accept flows into folk autocomplete.
	_, err := svc.Submit(t.Context(), suggest.SubmitInput{
		WorkID: "wabc123def456", Term: vocab.TermRef{Scheme: vocab.FolkScheme, ID: "Cozy Fantasy"},
		Type: suggest.TypeAdd, SupporterHash: "h",
	})
	if err != nil {
		t.Fatal(err)
	}
	// PROPOSED: invisible.
	rec := doJSON(t, h, http.MethodGet, "/v1/terms?scheme=folk&q=cozy", "", nil)
	var out struct {
		Terms []vocab.TermRef `json:"terms"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if len(out.Terms) != 0 {
		t.Fatalf("proposed term visible: %+v", out.Terms)
	}
	accept := map[string]any{"action": "acceptFolk", "folkTerm": "cozy fantasy"}
	if rec := doJSON(t, h, http.MethodPost, "/v1/terms", "lib-token", accept); rec.Code != http.StatusNoContent {
		t.Fatalf("accept folk: %d", rec.Code)
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/terms?scheme=folk&q=cozy", "", nil)
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if len(out.Terms) != 1 || out.Terms[0].ID != "cozy fantasy" {
		t.Fatalf("accepted terms = %+v", out.Terms)
	}
}

// TestQueueJoinsWorkTitles covers the read-time title join: a pipeline
// suggestion is created title-less, and the queue names its work from the
// work index -- a reviewer triages "should THIS BOOK gain this term", not a
// work id. A row that stored a title (the patron path keeps what the patron
// saw) is left alone.
func TestQueueJoinsWorkTitles(t *testing.T) {
	// The moderation harness with a REAL blob store + work index, so the
	// join has summaries to read.
	data, err := os.ReadFile("../vocab/testdata/authorities.nq")
	if err != nil {
		t.Fatal(err)
	}
	bs := blob.NewMem()
	_, _ = bs.Put(t.Context(), "a/x.nq", data, blob.PutOptions{})
	ix, err := vocab.Load(t.Context(), bs, "a/", nil)
	if err != nil {
		t.Fatal(err)
	}
	queue := suggest.New(store.NewMem(), ix, suggest.Caps{})
	if _, err := queue.PutPolicy(t.Context(), suggest.Policy{Enabled: true, FreeText: suggest.FreeTextAny}); err != nil {
		t.Fatal(err)
	}
	verifier := staffVerifier{
		"mod-token": {Email: "mod@example.org", Roles: []auth.Role{auth.RoleModerator}},
		"lib-token": {Email: "lib@example.org", Roles: []auth.Role{auth.RoleLibrarian}},
	}
	h := New(Deps{Blob: bs, DB: store.NewMem(), Suggest: queue, Vocab: ix, Verifier: verifier})
	seedAuditWork(t, bs, "wqtitle0001a", "", "", "zines", nil)
	if err := queue.PipelineSuggest(t.Context(), "wqtitle0001a",
		vocab.TermRef{Scheme: "homosaurus", ID: "https://homosaurus.org/v5/homoit0009999", Label: "Zines"}, 0.8); err != nil {
		t.Fatal(err)
	}
	// A patron-style row with its own stored title, for a work the index
	// also knows under a DIFFERENT (edited) title: the stored one wins.
	if _, err := queue.Submit(t.Context(), suggest.SubmitInput{
		WorkID: "wqtitle0001a",
		Term:   vocab.TermRef{Scheme: "homosaurus", ID: transURI, Label: "Other"},
		Type:   suggest.TypeAdd, SupporterHash: "h", WorkTitle: "Title The Patron Saw",
	}); err != nil {
		t.Fatal(err)
	}

	rec := request(t, h, http.MethodGet, "/v1/queue", "mod-token", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("queue = %d (%s)", rec.Code, rec.Body)
	}
	var page struct {
		Items []struct {
			WorkID    string `json:"workId"`
			WorkTitle string `json:"workTitle"`
			Term      struct{ ID string }
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 2 {
		t.Fatalf("items = %d (%s)", len(page.Items), rec.Body)
	}
	byTerm := map[string]string{}
	for _, it := range page.Items {
		byTerm[it.Term.ID] = it.WorkTitle
	}
	// seedAuditWork titles the work "T <workID>".
	if got := byTerm["https://homosaurus.org/v5/homoit0009999"]; got != "T wqtitle0001a" {
		t.Fatalf("pipeline row title = %q, want the index join", got)
	}
	if got := byTerm[transURI]; got != "Title The Patron Saw" {
		t.Fatalf("patron row title = %q, want the stored title kept", got)
	}
}

// TestQueueMinConfidence pins task 431: the floor hides PIPELINE rows below
// it while patron rows (no machine confidence) always pass; the deployment
// default applies when the request is silent; an explicit ?minConfidence
// overrides in BOTH directions (including 0 to see everything); and an
// unparseable value is a 400, never a silent no-op.
func TestQueueMinConfidence(t *testing.T) {
	h, bs, queue := newRecordsAPIWithQueue(t)
	seedAuditWork(t, bs, "wqconf0001a", "", "", "zines", nil)
	if err := queue.PipelineSuggest(t.Context(), "wqconf0001a",
		vocab.TermRef{Scheme: "homosaurus", ID: "urn:homo:women431", Label: "Women"}, 0.8); err != nil {
		t.Fatal(err)
	}
	if err := queue.PipelineSuggest(t.Context(), "wqconf0001a",
		vocab.TermRef{Scheme: "homosaurus", ID: "urn:homo:womyn431", Label: "Womyn"}, 0.6); err != nil {
		t.Fatal(err)
	}

	items := func(query string) []string {
		t.Helper()
		rec := request(t, h, http.MethodGet, "/v1/queue"+query, "mod-token", "", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("queue%s = %d (%s)", query, rec.Code, rec.Body)
		}
		var page struct {
			Items []struct {
				Term struct{ ID string }
			} `json:"items"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &page)
		out := make([]string, 0, len(page.Items))
		for _, it := range page.Items {
			out = append(out, it.Term.ID)
		}
		return out
	}

	// No floor configured, none requested: everything shows.
	if got := items(""); len(got) != 2 {
		t.Fatalf("unfiltered = %v, want both", got)
	}
	// A request floor hides the see-also tier.
	got := items("?minConfidence=0.7")
	if len(got) != 1 || got[0] != "urn:homo:women431" {
		t.Fatalf("floored = %v, want only the 0.8 row", got)
	}
	// Garbage is a 400, not a silent no-op.
	for _, bad := range []string{"?minConfidence=maybe", "?minConfidence=1.5", "?minConfidence=-0.1"} {
		if rec := request(t, h, http.MethodGet, "/v1/queue"+bad, "mod-token", "", nil); rec.Code != http.StatusBadRequest {
			t.Errorf("%s = %d, want 400", bad, rec.Code)
		}
	}
}

// TestQueueMinConfidenceDefaultAndOverride: the deployment floor applies
// when the request is silent, an explicit 0 overrides it back to
// everything, and patron rows pass the floor regardless.
func TestQueueMinConfidenceDefaultAndOverride(t *testing.T) {
	bs := blob.NewMem()
	db := store.NewMem()
	verifier := staffVerifier{
		"mod-token": {Email: "mod@example.org", Roles: []auth.Role{auth.RoleModerator}},
	}
	data, err := os.ReadFile("../vocab/testdata/authorities.nq")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = bs.Put(t.Context(), "a/x.nq", data, blob.PutOptions{})
	ix, err := vocab.Load(t.Context(), bs, "a/", nil)
	if err != nil {
		t.Fatal(err)
	}
	queue := suggest.New(db, ix, suggest.Caps{})
	if _, err := queue.PutPolicy(t.Context(), suggest.Policy{Enabled: true, FreeText: suggest.FreeTextAny}); err != nil {
		t.Fatal(err)
	}
	h := New(Deps{Blob: bs, DB: db, Suggest: queue, Vocab: ix, Verifier: verifier, QueueMinConfidence: 0.7})

	if err := queue.PipelineSuggest(t.Context(), "wqconf0002a",
		vocab.TermRef{Scheme: "homosaurus", ID: "urn:homo:seealso", Label: "See Also"}, 0.6); err != nil {
		t.Fatal(err)
	}
	// A patron row carries no machine confidence; the floor must not eat it.
	if _, err := queue.Submit(t.Context(), suggest.SubmitInput{
		WorkID: "wqconf0002a",
		Term:   vocab.TermRef{Scheme: "homosaurus", ID: transURI, Label: "Patron pick"},
		Type:   suggest.TypeAdd, SupporterHash: "h", WorkTitle: "T",
	}); err != nil {
		t.Fatal(err)
	}

	count := func(query string) int {
		t.Helper()
		rec := request(t, h, http.MethodGet, "/v1/queue"+query, "mod-token", "", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("queue%s = %d (%s)", query, rec.Code, rec.Body)
		}
		var page struct {
			Items []struct{ WorkID string } `json:"items"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &page)
		return len(page.Items)
	}
	if got := count(""); got != 1 {
		t.Fatalf("default floor: %d items, want 1 (patron row only)", got)
	}
	if got := count("?minConfidence=0"); got != 2 {
		t.Fatalf("explicit zero override: %d items, want everything", got)
	}
}
