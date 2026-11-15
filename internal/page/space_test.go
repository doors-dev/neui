package page

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/doors-dev/gox"
	"github.com/zeebo/xxh3"
	"neui.dev/internal/island"
)

func TestIslandPathIsCanonical(t *testing.T) {
	p := NewPage(context.Background())
	isl := island.NewIsland(gox.Elem(func(gox.Cursor) error { return nil }), func(slot gox.Comp) gox.Elem { return slot.Main() }, nil, nil, island.NewLibrary())
	t.Cleanup(isl.Unmount)
	p.space.children = []*space{{root: p, ctx: context.Background(), name: "isl", path: "isl", island: isl}}
	for _, raw := range []string{"isl", "isl."} {
		got, path, err := p.Island(context.Background(), raw)
		if err != nil {
			t.Fatal(err)
		}
		if got != isl || path != "isl" {
			t.Fatalf("%q: got path %q", raw, path)
		}
	}
}

func note(text string) *data {
	d := &data{Text: text}
	d.raw, _ = json.Marshal(map[string]string{"text": text})
	d.hash = xxh3.Hash(d.raw)
	return d
}

func tree(paths ...string) *page {
	p := NewPage(context.Background())
	for _, path := range paths {
		s := p.space
		for _, name := range strings.Split(path, ".") {
			i, found := s.search(name)
			if !found {
				c := &space{root: p, name: name, path: strings.TrimPrefix(s.path+"."+name, "."), desc: "the " + name}
				s.children = slices.Insert(s.children, i, c)
			}
			s = s.children[i]
		}
	}
	return p
}

func (p *page) at(path string) *space {
	s, err := p.space.find(strings.Split(path, "."))
	if err != nil {
		panic(err)
	}
	return s
}

func TestDigestTreatsDataAsSet(t *testing.T) {
	s := tree("cart").at("cart")
	s.data = []*data{note("a"), note("b")}
	before := s.digest()
	s.data = []*data{note("b"), note("a"), note("b")}
	if s.digest() != before {
		t.Fatal("a duplicate or a new order changed the digest")
	}
	if got := s.state().Data; len(got) != 2 || got[0].Text != "b" || got[1].Text != "a" {
		t.Fatalf("the state lists %+v, want b and a once each", got)
	}
	s.data = s.data[:2]
	if s.digest() != before {
		t.Fatal("removing one of two duplicates changed the digest")
	}
	s.data = s.data[:1]
	if s.digest() == before {
		t.Fatal("removing the last a left the digest as it was")
	}
}

func TestDiffShowsWhatChanged(t *testing.T) {
	p := tree("cart.items", "cart.total")
	s := p.at("cart")
	s.data = []*data{note("one")}
	old := s.snapshot(0)
	if d := s.diff(old); d != nil {
		t.Fatalf("an unchanged space diffs as %+v", d)
	}
	s.data = nil
	s.children = s.children[:1]
	s.children = append(s.children, &space{root: p, name: "zip", path: "cart.zip"})
	d := s.diff(old)
	if d.Data == nil || len(d.Data) != 0 || d.Tools != nil || d.Inputs != nil {
		t.Fatalf("the diff is %+v, want the emptied data list alone besides children", d)
	}
	if d.Children == nil || len(d.Children.Added) != 1 || d.Children.Added[0].Name != "zip" || !slices.Equal(d.Children.Removed, []string{"total"}) {
		t.Fatalf("the children diff is %+v, want zip added and total removed", d.Children)
	}
	raw, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"data":[],"children":{"added":[{"name":"zip","desc":""}],"removed":["total"]}}`; string(raw) != want {
		t.Fatalf("the diff encodes as %s, want %s", raw, want)
	}
}

func TestTreeMarksWhatItLeavesOut(t *testing.T) {
	var paths []string
	for i := range 25 {
		paths = append(paths, fmt.Sprintf("list.r%02d", i))
	}
	p := tree(append(paths, "a.b.c.d.e")...)
	root := p.space.tree(3)
	if root.More != 0 || len(root.Children) != 2 {
		t.Fatalf("the root shows %d children and more %d, want a and list", len(root.Children), root.More)
	}
	list := root.Children[1]
	if len(list.Children) != 20 || list.More != 5 || list.Children[19].Name != "r19" {
		t.Fatalf("list shows %d children and more %d, want the first 20 and more 5", len(list.Children), list.More)
	}
	c := root.Children[0].Children[0].Children[0]
	if c.Name != "c" || c.Children != nil || c.More != 1 {
		t.Fatalf("level 3 shows %+v, want c with more 1", c)
	}
	if alone := p.at("list").tree(0); alone.Name != "list" || alone.Children != nil || alone.More != 25 {
		t.Fatalf("depth 0 shows %+v, want the node alone with more 25", alone)
	}
}

func TestChangesListOneEntryPerSubtree(t *testing.T) {
	p := tree("a", "b.gone.deep", "c.x")
	before := map[string][4]uint64{}
	p.space.digests(before)
	p.at("a").data = []*data{note("new")}
	p.at("b").children = nil
	p.at("c").children = append(p.at("c").children, &space{root: p, name: "y", path: "c.y"})
	n := &space{root: p, name: "n", path: "n", desc: "fresh"}
	n.children = []*space{{root: p, name: "inner", path: "n.inner"}}
	p.space.children = append(p.space.children, n)
	raw, err := json.Marshal(p.space.changes(before, "c"))
	if err != nil {
		t.Fatal(err)
	}
	want := `[{"path":"","changed":["children"]},{"path":"a","changed":["data"]},{"path":"b","changed":["children"]},{"path":"b.gone","removed":true},{"path":"n","added":true,"desc":"fresh"}]`
	if string(raw) != want {
		t.Fatalf("the changes are %s, want %s: c's own new child is its diff's, n.inner and b.gone.deep are under their tops", raw, want)
	}
}

func TestChangesFoldPastFifty(t *testing.T) {
	var paths []string
	for i := range 30 {
		for j := range 5 {
			paths = append(paths, fmt.Sprintf("main.dash.p%02d.w%d", i, j))
		}
	}
	p := tree(append(paths, "header", "aside")...)
	before := map[string][4]uint64{}
	p.space.digests(before)
	for _, path := range paths {
		p.at(path).data = []*data{note(path)}
	}
	p.at("header").data = []*data{note("header")}
	for i := range 10 {
		p.space.children = append(p.space.children, &space{root: p, name: fmt.Sprintf("toast%d", i), path: fmt.Sprintf("toast%d", i)})
	}
	got := p.space.changes(before, "aside")
	if len(got) > 50 {
		t.Fatalf("%d entries, want at most 50", len(got))
	}
	spaces, added := 0, 0
	for _, c := range got {
		switch {
		case c.Added:
			added++
		case c.Spaces > 0:
			if !strings.HasPrefix(c.Path, "main.dash.p") || !slices.Contains(c.Changed, "children") {
				t.Fatalf("folded entry %+v, want a dash panel with children changed", c)
			}
			spaces += c.Spaces
		case c.Path != "header" && c.Path != "":
			t.Fatalf("unfolded entry %+v, want only the root, header and the toasts", c)
		}
		if c.Spaces > 0 && c.Spaces != 5 {
			t.Fatalf("folded entry %+v, want each panel folded with its 5 widgets", c)
		}
	}
	if spaces != 150 || added != 10 {
		t.Fatalf("the folded entries stand for %d spaces and %d toasts stay listed, want 150 and 10", spaces, added)
	}
}

func TestChangesFoldAddedWhenMoreThanTwentyFive(t *testing.T) {
	var paths []string
	for i := range 60 {
		paths = append(paths, fmt.Sprintf("orders.r%02d", i))
	}
	p := tree(paths...)
	before := map[string][4]uint64{}
	p.space.digests(before)
	orders := p.at("orders")
	orders.children = nil
	for i := range 60 {
		orders.children = append(orders.children, &space{root: p, name: fmt.Sprintf("k%02d", i), path: fmt.Sprintf("orders.k%02d", i)})
	}
	got := p.space.changes(before, "")
	if len(got) != 1 || got[0].Path != "orders" || got[0].Spaces != 120 || !slices.Equal(got[0].Changed, []string{"children"}) {
		t.Fatalf("a re-rendered list of 60 lists %+v, want one orders entry standing for 120 spaces", got)
	}
}
