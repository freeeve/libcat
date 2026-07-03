package project

import (
	"os"
	"testing"
)

// benchCorpus loads the benchmark corpus: point LCAT_BENCH_CATALOG at a real
// catalog.nq (e.g. the 5,659-work QLL corpus) for representative numbers;
// without it the benchmark skips rather than flattering itself on a toy file.
// LCAT_BENCH_PROVIDER overrides the corpus's feed provider (default
// "overdrive"; the playground corpus is "marc").
//
//	LCAT_BENCH_CATALOG=/path/catalog.nq go test ./project/ -bench . -benchmem
func benchCorpus(b *testing.B) ([]byte, string) {
	b.Helper()
	path := os.Getenv("LCAT_BENCH_CATALOG")
	if path == "" {
		b.Skip("LCAT_BENCH_CATALOG not set")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		b.Fatal(err)
	}
	provider := os.Getenv("LCAT_BENCH_PROVIDER")
	if provider == "" {
		provider = "overdrive"
	}
	return data, provider
}

func BenchmarkProject(b *testing.B) {
	data, provider := benchCorpus(b)
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		cat, err := Project(data, provider)
		if err != nil {
			b.Fatal(err)
		}
		if len(cat.Works) == 0 {
			b.Fatal("empty projection")
		}
	}
}

func BenchmarkFacets(b *testing.B) {
	data, provider := benchCorpus(b)
	cat, err := Project(data, provider)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		f := cat.Facets()
		if len(f.Languages) == 0 {
			b.Fatal("empty facets")
		}
	}
}
