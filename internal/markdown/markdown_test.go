package markdown

import (
	"bytes"
	"strings"
	"testing"
)

func TestBlock(t *testing.T) {
	for _, c := range []struct {
		name string
		src  string
		want string
	}{
		{"heading", "# Title", "<h1>Title</h1>\n"},
		{"hard wrap", "one\ntwo", "<p>one<br>\ntwo</p>\n"},
		{"emphasis", "**b** *i* ~~s~~ `c`", "<p><strong>b</strong> <em>i</em> <del>s</del> <code>c</code></p>\n"},
		{"task list", "- [x] done\n- [ ] todo", "<ul>\n<li><input checked=\"\" disabled=\"\" type=\"checkbox\"> done</li>\n<li><input disabled=\"\" type=\"checkbox\"> todo</li>\n</ul>\n"},
		{"fenced code", "```go\nx := \"<b>\"\n```", "<pre tabindex=\"0\"><code class=\"language-go\">x := &quot;&lt;b&gt;&quot;\n</code></pre>\n"},
		{"indented code", "    x < y", "<pre tabindex=\"0\"><code>x &lt; y\n</code></pre>\n"},
		{"table", "| A | `B` |\n|:--|--:|\n| 1 | 2 |", "<div role=\"region\" tabindex=\"0\" aria-label=\"Table: A, B\">\n<table>\n<thead>\n<tr>\n<th align=\"left\">A</th>\n<th align=\"right\"><code>B</code></th>\n</tr>\n</thead>\n<tbody>\n<tr>\n<td align=\"left\">1</td>\n<td align=\"right\">2</td>\n</tr>\n</tbody>\n</table>\n</div>\n"},
		{"table without titles", "| |\n|-|\n| 1 |", "<div role=\"region\" tabindex=\"0\" aria-label=\"Table\">"},
		{"raw inline html", "a <b onclick=\"x()\">b</b>", "<p>a <!-- raw HTML omitted -->b<!-- raw HTML omitted --></p>\n"},
		{"raw block html", "<script>alert(1)</script>", "<!-- raw HTML omitted -->\n"},
		{"https link", "[site](https://ex.com/a?b=1&c=2)", "<p><a href=\"https://ex.com/a?b=1&amp;c=2\" target=\"_blank\" rel=\"noopener noreferrer\">site</a></p>\n"},
		{"mailto link", "[mail](mailto:a@b.co)", "<p><a href=\"mailto:a@b.co\" target=\"_blank\" rel=\"noopener noreferrer\">mail</a></p>\n"},
		{"autolink", "see www.ex.com and a@b.co", "<p>see <a href=\"http://www.ex.com\" target=\"_blank\" rel=\"noopener noreferrer\">www.ex.com</a> and <a href=\"mailto:a@b.co\" target=\"_blank\" rel=\"noopener noreferrer\">a@b.co</a></p>\n"},
		{"javascript link", "[x](javascript:alert(1))", "<p>x</p>\n"},
		{"vbscript autolink", "<vbscript:msgbox>", "<p>vbscript:msgbox</p>\n"},
		{"data link", "[x](data:text/html;base64,PHNjcmlwdD4=)", "<p>x</p>\n"},
		{"relative link", "[x](notes.md)", "<p>x</p>\n"},
		{"file link", "[the report](file:///reports/q3%20final.pdf)", "<p><a href=\"file:///reports/q3%20final.pdf\">the report</a></p>\n"},
		{"file image", "![chart](file:///img/c.png \"Q3\")", "<p><img src=\"file:///img/c.png\" alt=\"chart\" title=\"Q3\"></p>\n"},
		{"data image", "![dot](data:image/png;base64,AAAA)", "<p><img src=\"data:image/png;base64,AAAA\" alt=\"dot\"></p>\n"},
		{"remote image", "![logo](https://ex.com/l.png)", "<p><a href=\"https://ex.com/l.png\" target=\"_blank\" rel=\"noopener noreferrer\">logo</a></p>\n"},
		{"svg data image", "![x](data:image/svg+xml,<svg>)", "<p>x</p>\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := string(Block(c.src))
			if !strings.HasPrefix(got, c.want) {
				t.Fatalf("Block(%q)\n got: %q\nwant: %q", c.src, got, c.want)
			}
		})
	}
}

func TestInline(t *testing.T) {
	for _, c := range []struct {
		name string
		src  string
		want string
	}{
		{"plain", "Ada", "Ada"},
		{"emphasis", "**b** *i* `c`", "<strong>b</strong> <em>i</em> <code>c</code>"},
		{"heading stays literal", "# not a heading", "# not a heading"},
		{"list stays literal", "- one\n1. two", "- one\n1. two"},
		{"quote stays literal", "> q", "&gt; q"},
		{"table stays literal", "a | b\n--|--\n1 | 2", "a | b\n--|--\n1 | 2"},
		{"paragraphs joined", "one\n\ntwo", "one<br>\ntwo"},
		{"link", "[site](https://ex.com)", "<a href=\"https://ex.com\" target=\"_blank\" rel=\"noopener noreferrer\">site</a>"},
		{"unsafe link", "[x](javascript:alert(1))", "x"},
		{"file link", "[q3.xlsx](file:///island/q3.xlsx)", "<a href=\"file:///island/q3.xlsx\">q3.xlsx</a>"},
		{"file image becomes a link", "![c](file:///c.png)", "<a href=\"file:///c.png\">c</a>"},
		{"raw html", "<i>x</i>", "<!-- raw HTML omitted -->x<!-- raw HTML omitted -->"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := string(Inline(c.src)); got != c.want {
				t.Fatalf("Inline(%q)\n got: %q\nwant: %q", c.src, got, c.want)
			}
		})
	}
}

func TestPlain(t *testing.T) {
	for _, c := range []struct {
		html []byte
		want string
	}{
		{Block("# Title\n\nFirst **bold** line\nnext"), "Title First bold line next"},
		{Block("- a\n- b\n\n| A | B |\n|-|-|\n| 1 | 2 |"), "a b A B 1 2"},
		{Inline("**Ada** `x` & <b>y</b>"), "Ada x & y"},
		{Inline("[the file](file:///a/b.csv)"), "the file"},
		{Inline("[file:///a/b.csv](file:///a/b.csv)"), "b.csv"},
		{Inline("<file:///a/b.csv>"), "b.csv"},
		{Inline(""), ""},
	} {
		if got := Plain(c.html); got != c.want {
			t.Errorf("Plain(%q) = %q, want %q", c.html, got, c.want)
		}
	}
}

func TestFilePath(t *testing.T) {
	for _, c := range []struct {
		href string
		want string
		ok   bool
	}{
		{"file:///island/a%20b.pdf", "island/a b.pdf", true},
		{"file:///agent_uploads/x.csv", "agent_uploads/x.csv", true},
		{"file:///a/./b/../c.txt", "a/c.txt", true},
		{"file:///", "", false},
		{"file:///%zz", "", false},
		{"https://ex.com/a", "", false},
		{"file://host/a", "", false},
	} {
		got, ok := FilePath(c.href)
		if got != c.want || ok != c.ok {
			t.Errorf("FilePath(%q) = %q, %v; want %q, %v", c.href, got, ok, c.want, c.ok)
		}
	}
}

func TestFileLabel(t *testing.T) {
	for _, c := range []struct {
		text, href, name, want string
	}{
		{"Q3 report", "file:///r/q3.pdf", "r/q3.pdf", "Q3 report"},
		{"  Q3 \n report ", "file:///r/q3.pdf", "r/q3.pdf", "Q3 report"},
		{"", "file:///r/q3.pdf", "r/q3.pdf", "q3.pdf"},
		{"file:///r/q3.pdf", "file:///r/q3.pdf", "r/q3.pdf", "q3.pdf"},
		{"r/q3.pdf", "file:///r/q3.pdf", "r/q3.pdf", "q3.pdf"},
	} {
		if got := FileLabel(c.text, c.href, c.name); got != c.want {
			t.Errorf("FileLabel(%q, %q, %q) = %q, want %q", c.text, c.href, c.name, got, c.want)
		}
	}
}

func TestConvertKeepsChatOutput(t *testing.T) {
	var buf bytes.Buffer
	if err := Convert([]byte("[f](file:///user_uploads/a.txt) [x](javascript:y) | A |\n|:-|\n| 1 |\n\n```\nc\n```\nline\nnext"), &buf); err != nil {
		t.Fatal(err)
	}
	want := "<p><a href=\"file:///user_uploads/a.txt\">f</a> <a href=\"\">x</a> | A |\n|:-|\n| 1 |</p>\n<pre><code>c\n</code></pre>\n<p>line\nnext</p>\n"
	if got := buf.String(); got != want {
		t.Fatalf("Convert\n got: %q\nwant: %q", got, want)
	}
}

func TestConvertLabelsTasks(t *testing.T) {
	var buf bytes.Buffer
	if err := Convert([]byte("- [x] ship **the** `fix`\n  today\n- [ ] say \"hi\"\n- [ ]"), &buf); err != nil {
		t.Fatal(err)
	}
	want := "<ul>\n<li><input checked=\"\" disabled=\"\" type=\"checkbox\" aria-label=\"ship the fix today\"> ship <strong>the</strong> <code>fix</code>\ntoday</li>\n<li><input disabled=\"\" type=\"checkbox\" aria-label=\"say &quot;hi&quot;\"> say &quot;hi&quot;</li>\n<li><input disabled=\"\" type=\"checkbox\"> </li>\n</ul>\n"
	if got := buf.String(); got != want {
		t.Fatalf("Convert\n got: %q\nwant: %q", got, want)
	}
	for src, label := range map[string]string{
		"- [ ] Tom &amp; Jerry":             `aria-label="Tom &amp; Jerry"`,
		"- [ ] use \\*args\\*":              `aria-label="use *args*"`,
		"- [ ] run `a\\*b`":                 `aria-label="run a\*b"`,
		"- [ ] see <https://example.com>":   `aria-label="see https://example.com"`,
		"- [ ] see https://example.com now": `aria-label="see https://example.com now"`,
	} {
		buf.Reset()
		if err := Convert([]byte(src), &buf); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(buf.String(), label) {
			t.Fatalf("%q renders %q, want the label as the user sees the text: %s", src, buf.String(), label)
		}
	}
}
