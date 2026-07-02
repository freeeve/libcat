package export

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	codexbf "github.com/freeeve/libcodex/bibframe"

	"github.com/freeeve/libcatalog/bibframe"
	"github.com/freeeve/libcatalog/ingest"
	"github.com/freeeve/libcatalog/ingest/marc"
	"github.com/freeeve/libcatalog/storage"
	"github.com/freeeve/libcatalog/storage/blob"

	"github.com/freeeve/libcatalog/backend/store"
)

const marcSample = "../../ingest/overdrive/testdata/marc-express/od-sample-ebook.mrc"

// buildFixtureTree ingests the vendored MARC Express sample into a temp dir
// through the real pipeline, then mirrors the grain tree into a MemStore.
func buildFixtureTree(t *testing.T) (blob.Store, []string) {
	t.Helper()
	dir := t.TempDir()
	prov, err := marc.New(ingest.Config{Source: marcSample})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ingest.Run(prov, dir); err != nil {
		t.Fatal(err)
	}
	bs := blob.NewMem()
	var workIDs []string
	err = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".nq") || d.Name() == "catalog.nq" {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		if _, err := bs.Put(t.Context(), filepath.ToSlash(rel), data, blob.PutOptions{}); err != nil {
			return err
		}
		workIDs = append(workIDs, strings.TrimSuffix(d.Name(), ".nq"))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(workIDs) == 0 {
		t.Fatal("no grains built")
	}
	return bs, workIDs
}

func newService(t *testing.T, bs blob.Store) *Service {
	t.Helper()
	svc, err := New(store.NewMem(), bs, "marc", []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

func TestExportMARCRoundTrips(t *testing.T) {
	bs, workIDs := buildFixtureTree(t)
	svc := newService(t, bs)
	job, err := svc.Create(t.Context(), "lib@example.org", FormatMARC, Selection{WorkIDs: workIDs})
	if err != nil {
		t.Fatal(err)
	}
	if job.Status != StatusDone || job.Records < len(workIDs) {
		t.Fatalf("job = %+v", job)
	}
	out, err := svc.Open(t.Context(), job)
	if err != nil {
		t.Fatal(err)
	}
	// The exported .mrc re-parses through the same reader the ingest uses.
	recs, err := bibframe.ReadMARC(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("re-parse exported MARC: %v", err)
	}
	if len(recs) != job.Records {
		t.Fatalf("re-parsed %d records, job says %d", len(recs), job.Records)
	}
}

func TestExportNQuadsMatchesSerialize(t *testing.T) {
	bs, workIDs := buildFixtureTree(t)
	svc := newService(t, bs)
	job, err := svc.Create(t.Context(), "lib@example.org", FormatNQuads, Selection{WorkIDs: workIDs})
	if err != nil {
		t.Fatal(err)
	}
	out, err := svc.Open(t.Context(), job)
	if err != nil {
		t.Fatal(err)
	}
	// A full-selection N-Quads export equals SerializeGrains over the same
	// tree (mirror the store back to disk and compare).
	dir := t.TempDir()
	for entry, err := range bs.List(t.Context(), "data/works/") {
		if err != nil {
			t.Fatal(err)
		}
		data, _, _ := bs.Get(t.Context(), entry.Path)
		full := filepath.Join(dir, filepath.FromSlash(entry.Path))
		_ = os.MkdirAll(filepath.Dir(full), 0o755)
		_ = os.WriteFile(full, data, 0o644)
	}
	sink := t.TempDir()
	if _, err := bibframe.SerializeGrains(dir, storage.Dir(sink)); err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join(sink, "catalog.nq"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, want) {
		t.Fatalf("export (%d bytes) != SerializeGrains (%d bytes)", len(out), len(want))
	}
	// And it parses as BIBFRAME (editorial-inclusive corpus).
	if _, err := codexbf.Decode(out); err != nil {
		t.Fatalf("exported corpus does not decode: %v", err)
	}
}

func TestExportCSVAndJSONLD(t *testing.T) {
	bs, workIDs := buildFixtureTree(t)
	svc := newService(t, bs)

	job, err := svc.Create(t.Context(), "lib@example.org", FormatCSV, Selection{All: true})
	if err != nil {
		t.Fatal(err)
	}
	// All-selections queue; run the worker.
	if job.Status != StatusQueued {
		t.Fatalf("all-selection ran in-request: %+v", job)
	}
	if ran, err := svc.RunQueued(t.Context()); err != nil || ran != 1 {
		t.Fatalf("RunQueued = %d, %v", ran, err)
	}
	job, err = svc.Get(t.Context(), "lib@example.org", job.ID, false)
	if err != nil || job.Status != StatusDone {
		t.Fatalf("job after worker = %+v, %v", job, err)
	}
	out, _ := svc.Open(t.Context(), job)
	rows, err := csv.NewReader(bytes.NewReader(out)).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != len(workIDs)+1 || rows[0][0] != "id" || rows[0][1] != "title" {
		t.Fatalf("csv rows = %d (want %d+header), header %v", len(rows), len(workIDs), rows[0])
	}

	job, err = svc.Create(t.Context(), "lib@example.org", FormatJSONLD, Selection{WorkIDs: workIDs[:2]})
	if err != nil || job.Status != StatusDone {
		t.Fatalf("jsonld job = %+v, %v", job, err)
	}
	out, _ = svc.Open(t.Context(), job)
	var docs []json.RawMessage
	if err := json.Unmarshal(out, &docs); err != nil || len(docs) < 2 {
		t.Fatalf("jsonld = %d docs, %v", len(docs), err)
	}
}

func TestDownloadTokens(t *testing.T) {
	bs, workIDs := buildFixtureTree(t)
	svc := newService(t, bs)
	now := time.Now()
	svc.now = func() time.Time { return now }
	job, err := svc.Create(t.Context(), "lib@example.org", FormatNQuads, Selection{WorkIDs: workIDs[:1]})
	if err != nil {
		t.Fatal(err)
	}
	// MemStore is no Signer: the URL is the token route.
	url, err := svc.DownloadURL(t.Context(), job)
	if err != nil || !strings.HasPrefix(url, "/v1/exports/"+job.ID+"/download?token=") {
		t.Fatalf("url = %q, %v", url, err)
	}
	token := strings.TrimPrefix(url, "/v1/exports/"+job.ID+"/download?token=")
	if !svc.VerifyToken(job, token) {
		t.Fatal("fresh token rejected")
	}
	if svc.VerifyToken(job, token+"0") {
		t.Fatal("tampered token accepted")
	}
	// Expiry kills both the link and the token.
	now = now.Add(48 * time.Hour)
	if _, err := svc.DownloadURL(t.Context(), job); err == nil {
		t.Fatal("expired job linked")
	}
	if svc.VerifyToken(job, token) {
		t.Fatal("expired token accepted")
	}
}

func TestJobVisibility(t *testing.T) {
	bs, workIDs := buildFixtureTree(t)
	svc := newService(t, bs)
	job, err := svc.Create(t.Context(), "lib@example.org", FormatNQuads, Selection{WorkIDs: workIDs[:1]})
	if err != nil {
		t.Fatal(err)
	}
	// Another librarian cannot see it; an admin can.
	if _, err := svc.Get(t.Context(), "other@example.org", job.ID, false); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-user get: %v", err)
	}
	if _, err := svc.Get(t.Context(), "other@example.org", job.ID, true); err != nil {
		t.Fatalf("admin get: %v", err)
	}
	jobs, err := svc.List(t.Context(), "lib@example.org")
	if err != nil || len(jobs) != 1 {
		t.Fatalf("list = %+v, %v", jobs, err)
	}
	// Validation.
	if _, err := svc.Create(t.Context(), "x", Format("xml"), Selection{All: true}); err == nil {
		t.Fatal("unknown format accepted")
	}
	if _, err := svc.Create(t.Context(), "x", FormatMARC, Selection{}); err == nil {
		t.Fatal("empty selection accepted")
	}
	if _, err := svc.Create(t.Context(), "x", FormatMARC, Selection{All: true, WorkIDs: []string{"w1"}}); err == nil {
		t.Fatal("conflicting selection accepted")
	}
}
