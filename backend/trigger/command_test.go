package trigger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCommandRunsWithChangedPaths(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "seen.txt")
	c := &Command{Cmd: `printf '%s' "$LCAT_EVENT_KIND:$LCAT_CHANGED_PATHS" > seen.txt`, Dir: dir}
	event := Event{Kind: "grains-changed", Paths: []string{"data/works/aa/w1.nq", "data/works/bb/w2.nq"}, At: time.Now()}
	if err := c.Notify(t.Context(), event); err != nil {
		t.Fatalf("Notify: %v", err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	want := "grains-changed:data/works/aa/w1.nq\ndata/works/bb/w2.nq"
	if string(got) != want {
		t.Fatalf("command saw %q, want %q", got, want)
	}
}

func TestCommandFailureSurfaces(t *testing.T) {
	c := &Command{Cmd: "exit 3"}
	if err := c.Notify(t.Context(), Event{Kind: "x"}); err == nil {
		t.Fatal("non-zero exit swallowed")
	}
}

func TestFanout(t *testing.T) {
	dir := t.TempDir()
	okCmd := &Command{Cmd: "touch ran.txt", Dir: dir}
	failing := &Command{Cmd: "exit 1"}
	err := Fanout{failing, okCmd}.Notify(t.Context(), Event{Kind: "x"})
	if err == nil {
		t.Fatal("fanout dropped the failure")
	}
	// The failing notifier did not starve the second one.
	if _, statErr := os.Stat(filepath.Join(dir, "ran.txt")); statErr != nil {
		t.Fatalf("second notifier did not run: %v", statErr)
	}
	if !strings.Contains(err.Error(), "rebuild command") {
		t.Fatalf("err = %v", err)
	}
}
