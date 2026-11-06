package page

import (
	"context"
	"testing"

	"github.com/doors-dev/gox"
	"neui.dev/internal/island"
)

func TestIslandPathIsCanonical(t *testing.T) {
	p := NewPage(context.Background())
	isl := island.NewIsland(gox.Elem(func(gox.Cursor) error { return nil }), func(slot gox.Comp) gox.Elem { return slot.Main() }, nil, nil, island.NewLibrary())
	t.Cleanup(isl.Unmount)
	p.space.children = []*space{{root: p, ctx: context.Background(), name: "isl", path: "isl", seq: 7, island: isl}}
	for _, raw := range []string{"isl", "isl."} {
		got, seq, path, err := p.island(context.Background(), raw)
		if err != nil {
			t.Fatal(err)
		}
		if got != isl || seq != 7 || path != "isl" {
			t.Fatalf("%q: got seq %d path %q", raw, seq, path)
		}
	}
}
