package island

import (
	"slices"
	"strings"
	"testing"
)

func TestSpecIntroIsLogsThenReference(t *testing.T) {
	i, ctx := newTestIsland(t)
	spec, ends, items := i.Spec(ctx, "board")
	if !slices.Equal(items, []string{"logs and files", "reading the reference"}) || len(ends) != 2 || ends[1] != len(spec) {
		t.Fatalf("items %v ending at %v of %d bytes, want logs and files, then reading the reference:\n%s", items, ends, len(spec), spec)
	}
	logs := spec[:ends[0]]
	if !strings.HasPrefix(logs, "Logs and files\n") || !strings.Contains(logs, " "+logFile("board")+", ") || !strings.Contains(logs, " "+islandDir("board")+", ") || !strings.HasSuffix(logs, "\n") {
		t.Fatalf("the logs and files item reads:\n%s", logs)
	}
	if spec[ends[0]:] != specReference || !strings.HasPrefix(specReference, "\nReading the reference\n") {
		t.Fatalf("the reading the reference item reads:\n%s", spec[ends[0]:])
	}
	if strings.Contains(spec, "Limits") {
		t.Fatalf("the intro still carries a limits line:\n%s", spec)
	}
}
