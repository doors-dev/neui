package page

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
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
	got, path, err := p.Island(context.Background(), "isl")
	if err != nil {
		t.Fatal(err)
	}
	if got != isl || path != "isl" {
		t.Fatalf("got path %q", path)
	}
	for _, raw := range []string{"isl.", ".isl", "isl..x"} {
		if _, _, err := p.Island(context.Background(), raw); !errors.Is(err, errSpaceNotFound) {
			t.Fatalf("%q: got %v, want no such space", raw, err)
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
	p.restamp()
	return p
}

func (p *page) restamp() {
	var walk func(s *space)
	walk = func(s *space) {
		s.digest = s.fresh()
		for _, c := range s.children {
			walk(c)
		}
	}
	walk(p.space)
}

func (p *page) at(path string) *space {
	s, err := p.space.find(path)
	if err != nil {
		panic(err)
	}
	return s
}

func TestDigestTreatsDataAsSet(t *testing.T) {
	s := tree("cart").at("cart")
	a, b := note("a"), note("b")
	s.count(2, a.hash, 1)
	s.count(2, b.hash, 1)
	before := s.digest
	s.count(2, b.hash, -1)
	s.count(2, a.hash, -1)
	s.count(2, b.hash, 1)
	s.count(2, a.hash, 1)
	if s.digest != before {
		t.Fatal("adding the same data in another order changed the digest")
	}
	s.count(2, b.hash, 1)
	if s.digest != before {
		t.Fatal("a duplicate changed the digest")
	}
	s.data = []*data{b, a, b}
	if got := s.state().Data; len(got) != 2 || got[0].Text != "b" || got[1].Text != "a" {
		t.Fatalf("the state lists %+v, want b and a once each", got)
	}
	s.count(2, b.hash, -1)
	if s.digest != before {
		t.Fatal("removing one of two duplicates changed the digest")
	}
	s.count(2, a.hash, -1)
	if s.digest == before {
		t.Fatal("removing the last a left the digest as it was")
	}
}

func TestDiffShowsWhatChanged(t *testing.T) {
	p := tree("cart.items", "cart.total")
	s := p.at("cart")
	s.data = []*data{note("one")}
	p.restamp()
	old := s.snapshot()
	if d := s.diff(old); d == nil || !reflect.ValueOf(*d).IsZero() {
		t.Fatalf("an unchanged space diffs as %+v, want an empty diff", d)
	}
	s.data = nil
	s.children = s.children[:1]
	s.children = append(s.children, &space{root: p, name: "zip", path: "cart.zip"})
	p.restamp()
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

func TestDiffAndChangesShowToolsAndInputs(t *testing.T) {
	kit := func(name string, version int) *tool {
		k := &tool{Name: name}
		k.raw, _ = json.Marshal(map[string]any{"name": name, "version": version})
		k.hash = xxh3.Hash(k.raw)
		return k
	}
	field := func(id string, rights Rights) input {
		in := input{ID: id, Rights: rights}
		in.raw, _ = json.Marshal(map[string]any{"id": id, "rights": rights})
		in.hash = xxh3.Hash(in.raw)
		return in
	}
	p := tree("form", "tools", "inputs", "all")
	form := p.at("form")
	form.tools = []*tool{kit("a", 1), kit("b", 1)}
	form.list = []input{field("i1", RW), field("i2", RW)}
	p.at("tools").tools = []*tool{kit("t", 1)}
	p.at("inputs").list = []input{field("x", RW)}
	p.at("all").tools = []*tool{kit("t", 1)}
	p.restamp()
	old := form.snapshot()
	before := map[string][4]uint64{}
	p.space.digests(before)
	form.tools = []*tool{kit("b", 2), kit("c", 1)}
	form.list = []input{field("i2", R), field("i3", RW)}
	p.at("tools").tools = []*tool{kit("t", 2)}
	p.at("inputs").list = nil
	all := p.at("all")
	all.tools = nil
	all.list = []input{field("y", RW)}
	all.data = []*data{note("new")}
	p.restamp()
	raw, err := json.Marshal(form.diff(old))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"tools":{"added":[{"name":"b","version":2},{"name":"c","version":1}],"removed":["a","b"]},"inputs":{"added":[{"id":"i2","rights":"R"},{"id":"i3","rights":"RW"}],"removed":["i1","i2"]}}`
	if string(raw) != want {
		t.Fatalf("the diff is %s, want %s: a changed tool or input is removed and added again", raw, want)
	}
	raw, err = json.Marshal(p.space.changes(before, "form"))
	if err != nil {
		t.Fatal(err)
	}
	want = `[{"path":"all","changed":["tools","inputs","data"]},{"path":"inputs","changed":["inputs"]},{"path":"tools","changed":["tools"]}]`
	if string(raw) != want {
		t.Fatalf("the changes are %s, want %s", raw, want)
	}
}

func TestFailureUnwrapsItsError(t *testing.T) {
	err := fmt.Errorf("jam: %w", errToolNotFound)
	f := failure{result: &report{Diff: &diff{}}, err: err}
	if !errors.Is(f, errToolNotFound) || f.Error() != err.Error()+"; "+failedAfterRunMessage {
		t.Fatalf("the failure reads %q and unwraps to %v, want the error it carries and the note that the call ran", f.Error(), errors.Unwrap(f))
	}
}

func TestCleanedInputIsNoDuplicate(t *testing.T) {
	s := tree("form").at("form")
	s.ctx = context.Background()
	gone, cancel := context.WithCancel(context.Background())
	cancel()
	s.list = []input{{ID: "a1", ctx: gone, elementID: "form-1", Rights: RW}, {ID: "a1", ctx: context.Background(), elementID: "form-2", Rights: RW}}
	in, err := s.getInput("a1", "click")
	if err != nil || in.elementID != "form-2" {
		t.Fatalf("the input next to its cleaned twin answered %q, %v, want the live one", in.elementID, err)
	}
	s.list[0].ctx = context.Background()
	if _, err := s.getInput("a1", "click"); !errors.Is(err, errRefusedDuplicate) || !strings.HasPrefix(err.Error(), `input "a1" in space "form": `) {
		t.Fatalf("two live inputs with one id answered %v, want the duplicate refusal for the space", err)
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

func TestTreeCountsWhatEachSpaceHolds(t *testing.T) {
	p := tree("form")
	form := p.at("form")
	form.tools = []*tool{{Name: "a"}, {Name: "b"}}
	field := input{ID: "i1"}
	field.raw, _ = json.Marshal(map[string]any{"id": "i1"})
	field.hash = xxh3.Hash(field.raw)
	form.list = []input{field, field}
	form.data = []*data{note("x"), note("y"), note("x")}
	got := p.space.tree(1).Children[0]
	if got.Tools != 2 || got.Inputs != 1 || got.Data != 2 {
		t.Fatalf("form counts %d tools, %d inputs and %d data, want 2, 1 and 2: inputs and data that cannot be told apart are listed once", got.Tools, got.Inputs, got.Data)
	}
	if root := p.space.tree(0); root.Tools != 0 || root.Inputs != 0 || root.Data != 0 {
		t.Fatalf("the empty root counts %+v, want nothing", root)
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
	p.restamp()
	raw, err := json.Marshal(p.space.changes(before, "c"))
	if err != nil {
		t.Fatal(err)
	}
	want := `[{"path":"a","changed":["data"]},{"path":"b.gone","removed":true},{"path":"n","added":true,"desc":"fresh"}]`
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
	p.restamp()
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
		case c.Path != "header":
			t.Fatalf("unfolded entry %+v, want only header and the toasts", c)
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
	p.restamp()
	got := p.space.changes(before, "")
	if len(got) != 1 || got[0].Path != "orders" || got[0].Spaces != 120 || !slices.Equal(got[0].Changed, []string{"children"}) {
		t.Fatalf("a re-rendered list of 60 lists %+v, want one orders entry standing for 120 spaces", got)
	}
}

func TestChangesFoldLargestClustersFirst(t *testing.T) {
	var paths []string
	for i := range 10 {
		paths = append(paths, fmt.Sprintf("a.x%d", i))
	}
	for i := range 3 {
		paths = append(paths, fmt.Sprintf("b.y%d", i))
	}
	for i := range 40 {
		paths = append(paths, fmt.Sprintf("s%02d", i))
	}
	p := tree(paths...)
	before := map[string][4]uint64{}
	p.space.digests(before)
	for _, path := range paths {
		p.at(path).data = []*data{note(path)}
	}
	p.restamp()
	got := p.space.changes(before, "")
	var folded, listed []string
	for _, c := range got {
		if c.Spaces > 0 {
			folded = append(folded, fmt.Sprintf("%s:%d", c.Path, c.Spaces))
		} else if strings.HasPrefix(c.Path, "b.") {
			listed = append(listed, c.Path)
		}
	}
	if len(got) != 44 || !slices.Equal(folded, []string{"a:10"}) || len(listed) != 3 {
		t.Fatalf("53 changes folded to %d entries, folded %v and listed %v from b, want only a's 10 folded and b's 3 listed", len(got), folded, listed)
	}
}

func TestChangesKeepTwentyFiveAdded(t *testing.T) {
	for _, n := range []int{25, 26} {
		var paths []string
		for i := range 26 {
			paths = append(paths, fmt.Sprintf("old.r%02d", i))
		}
		p := tree(append(paths, "list")...)
		before := map[string][4]uint64{}
		p.space.digests(before)
		for _, path := range paths {
			p.at(path).data = []*data{note(path)}
		}
		list := p.at("list")
		for i := range n {
			list.children = append(list.children, &space{root: p, name: fmt.Sprintf("k%02d", i), path: fmt.Sprintf("list.k%02d", i)})
		}
		p.restamp()
		got := p.space.changes(before, "")
		added := 0
		for _, c := range got {
			if c.Added {
				added++
			}
		}
		want := map[int][2]int{25: {26, 25}, 26: {2, 0}}[n]
		if len(got) != want[0] || added != want[1] {
			t.Fatalf("%d added and 26 changed folded to %d entries with %d added listed, want %d and %d", n, len(got), added, want[0], want[1])
		}
	}
}
