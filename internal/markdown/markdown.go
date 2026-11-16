package markdown

import (
	"bufio"
	"bytes"
	"html"
	"io"
	"net/url"
	"path"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
	gmparser "github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	gmhtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
	xhtml "golang.org/x/net/html"
)

const FileScheme = "file:///"

var chat = goldmark.New(
	goldmark.WithExtensions(extension.GFM, extension.TaskList),
	goldmark.WithRendererOptions(
		renderer.WithNodeRenderers(util.Prioritized(fileLinkRenderer{}, 100), util.Prioritized(taskRenderer{}, 100)),
	),
)

var block = goldmark.New(
	goldmark.WithExtensions(
		extension.Linkify,
		extension.NewTable(extension.WithTableCellAlignMethod(extension.TableCellAlignAttribute)),
		extension.Strikethrough,
		extension.TaskList,
	),
	goldmark.WithRendererOptions(
		gmhtml.WithHardWraps(),
		renderer.WithNodeRenderers(util.Prioritized(islandRenderer{}, 100)),
	),
)

var inline = goldmark.New(
	goldmark.WithParser(gmparser.NewParser(
		gmparser.WithBlockParsers(util.Prioritized(gmparser.NewParagraphParser(), 1000)),
		gmparser.WithInlineParsers(gmparser.DefaultInlineParsers()...),
		gmparser.WithParagraphTransformers(gmparser.DefaultParagraphTransformers()...),
	)),
	goldmark.WithExtensions(extension.Linkify, extension.Strikethrough),
	goldmark.WithRendererOptions(
		renderer.WithNodeRenderers(util.Prioritized(islandRenderer{inline: true}, 100)),
	),
)

func Convert(src []byte, w io.Writer, opts ...gmparser.ParseOption) error {
	return chat.Convert(src, w, opts...)
}

func Block(src string) []byte {
	return convert(block, src)
}

func Inline(src string) []byte {
	return convert(inline, src)
}

func convert(md goldmark.Markdown, src string) []byte {
	var buf bytes.Buffer
	if err := md.Convert([]byte(src), &buf); err != nil {
		buf.Reset()
		buf.WriteString("<pre>")
		buf.WriteString(html.EscapeString(src))
		buf.WriteString("</pre>\n")
	}
	return buf.Bytes()
}

func FilePath(href string) (string, bool) {
	rest, ok := strings.CutPrefix(href, FileScheme)
	if !ok {
		return "", false
	}
	name, err := url.PathUnescape(rest)
	if err != nil || name == "" {
		return "", false
	}
	return path.Clean(name), true
}

func FileLabel(text, href, name string) string {
	text = strings.Join(strings.Fields(text), " ")
	if text == "" || text == href || text == name || strings.HasPrefix(text, FileScheme) {
		return path.Base(name)
	}
	return text
}

func Plain(data []byte) string {
	var b strings.Builder
	z := xhtml.NewTokenizer(bytes.NewReader(data))
	for {
		switch z.Next() {
		case xhtml.ErrorToken:
			return strings.Join(strings.Fields(b.String()), " ")
		case xhtml.TextToken:
			b.Write(z.Text())
		case xhtml.StartTagToken, xhtml.EndTagToken, xhtml.SelfClosingTagToken:
			name, hasAttr := z.TagName()
			switch string(name) {
			case "a", "strong", "em", "code", "del", "span":
			default:
				b.WriteByte(' ')
			}
			if string(name) != "a" || !hasAttr {
				continue
			}
			href := ""
			for {
				k, v, more := z.TagAttr()
				if string(k) == "href" {
					href = string(v)
				}
				if !more {
					break
				}
			}
			file, ok := FilePath(href)
			if !ok {
				continue
			}
			var text strings.Builder
		link:
			for {
				switch z.Next() {
				case xhtml.ErrorToken:
					break link
				case xhtml.TextToken:
					text.Write(z.Text())
				case xhtml.EndTagToken:
					if t, _ := z.TagName(); string(t) == "a" {
						break link
					}
				}
			}
			b.WriteString(FileLabel(text.String(), href, file))
		}
	}
}

type fileLinkRenderer struct{}

func (fileLinkRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindLink, renderLink)
	reg.Register(ast.KindImage, renderImage)
}

func isFileURL(dest []byte) bool {
	return bytes.HasPrefix(dest, []byte(FileScheme))
}

func renderLink(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*ast.Link)
	if entering {
		w.WriteString(`<a href="`)
		dest := util.URLEscape(n.Destination, true)
		if isFileURL(dest) || !gmhtml.IsDangerousURL(dest) {
			w.Write(util.EscapeHTML(dest))
		}
		w.WriteByte('"')
		if n.Title != nil {
			w.WriteString(` title="`)
			w.Write(util.EscapeHTML(n.Title))
			w.WriteByte('"')
		}
		w.WriteByte('>')
	} else {
		w.WriteString("</a>")
	}
	return ast.WalkContinue, nil
}

func isInlineImage(dest []byte) bool {
	if isFileURL(dest) {
		return true
	}
	return len(dest) >= 11 && bytes.EqualFold(dest[:11], []byte("data:image/")) && !gmhtml.IsDangerousURL(dest)
}

func renderImage(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*ast.Image)
	dest := util.URLEscape(n.Destination, true)
	if !isInlineImage(dest) {
		if !entering {
			w.WriteString("</a>")
			return ast.WalkContinue, nil
		}
		w.WriteString(`<a href="`)
		if !gmhtml.IsDangerousURL(dest) {
			w.Write(util.EscapeHTML(dest))
		}
		w.WriteByte('"')
		if n.Title != nil {
			w.WriteString(` title="`)
			w.Write(util.EscapeHTML(n.Title))
			w.WriteByte('"')
		}
		w.WriteByte('>')
		if n.FirstChild() == nil {
			w.Write(util.EscapeHTML(n.Destination))
		}
		return ast.WalkContinue, nil
	}
	if !entering {
		return ast.WalkSkipChildren, nil
	}
	w.WriteString(`<img src="`)
	w.Write(util.EscapeHTML(dest))
	w.WriteString(`" alt="`)
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if t, ok := c.(*ast.Text); ok {
			w.Write(util.EscapeHTML(t.Value(source)))
		}
	}
	w.WriteByte('"')
	if n.Title != nil {
		w.WriteString(` title="`)
		w.Write(util.EscapeHTML(n.Title))
		w.WriteByte('"')
	}
	w.WriteString(">")
	return ast.WalkSkipChildren, nil
}

type taskRenderer struct{}

func (taskRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(east.KindTaskCheckBox, renderTaskCheckBox)
}

func renderTaskCheckBox(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	var b bytes.Buffer
	text := bufio.NewWriter(&b)
	for n := node.NextSibling(); n != nil; n = n.NextSibling() {
		ast.Walk(n, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
			if !entering {
				return ast.WalkContinue, nil
			}
			switch t := n.(type) {
			case *ast.CodeSpan:
				for c := t.FirstChild(); c != nil; c = c.NextSibling() {
					if s, ok := c.(*ast.Text); ok {
						gmhtml.DefaultWriter.RawWrite(text, s.Value(source))
					}
				}
				return ast.WalkSkipChildren, nil
			case *ast.Text:
				gmhtml.DefaultWriter.Write(text, t.Value(source))
				if t.SoftLineBreak() || t.HardLineBreak() {
					text.WriteByte(' ')
				}
			case *ast.String:
				gmhtml.DefaultWriter.Write(text, t.Value)
			case *ast.AutoLink:
				gmhtml.DefaultWriter.Write(text, t.Label(source))
			}
			return ast.WalkContinue, nil
		})
	}
	text.Flush()
	w.WriteString("<input")
	if node.(*east.TaskCheckBox).IsChecked {
		w.WriteString(` checked=""`)
	}
	w.WriteString(` disabled="" type="checkbox"`)
	if label := strings.Join(strings.Fields(b.String()), " "); label != "" {
		w.WriteString(` aria-label="`)
		w.WriteString(label)
		w.WriteByte('"')
	}
	w.WriteString("> ")
	return ast.WalkContinue, nil
}

type islandRenderer struct {
	inline bool
}

func (r islandRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindLink, r.renderLink)
	reg.Register(ast.KindAutoLink, r.renderAutoLink)
	reg.Register(ast.KindImage, r.renderImage)
	if r.inline {
		reg.Register(ast.KindParagraph, r.renderParagraph)
		return
	}
	reg.Register(ast.KindCodeBlock, r.renderCodeBlock)
	reg.Register(ast.KindFencedCodeBlock, r.renderCodeBlock)
	reg.Register(east.KindTable, r.renderTable)
}

type islandLink int

const (
	islandLinkNone islandLink = iota
	islandLinkFile
	islandLinkExternal
)

func islandLinkOf(dest []byte) islandLink {
	lower := bytes.ToLower(dest)
	switch {
	case isFileURL(dest):
		return islandLinkFile
	case gmhtml.IsDangerousURL(dest):
		return islandLinkNone
	case bytes.HasPrefix(lower, []byte("https://")), bytes.HasPrefix(lower, []byte("http://")), bytes.HasPrefix(lower, []byte("mailto:")):
		return islandLinkExternal
	}
	return islandLinkNone
}

func islandOpenLink(w util.BufWriter, dest []byte, title []byte) bool {
	kind := islandLinkOf(dest)
	if kind == islandLinkNone {
		return false
	}
	w.WriteString(`<a href="`)
	w.Write(util.EscapeHTML(dest))
	w.WriteByte('"')
	if kind == islandLinkExternal {
		w.WriteString(` target="_blank" rel="noopener noreferrer"`)
	}
	if title != nil {
		w.WriteString(` title="`)
		w.Write(util.EscapeHTML(title))
		w.WriteByte('"')
	}
	w.WriteByte('>')
	return true
}

func (r islandRenderer) renderLink(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*ast.Link)
	dest := util.URLEscape(n.Destination, true)
	if entering {
		islandOpenLink(w, dest, n.Title)
	} else if islandLinkOf(dest) != islandLinkNone {
		w.WriteString("</a>")
	}
	return ast.WalkContinue, nil
}

func (r islandRenderer) renderAutoLink(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*ast.AutoLink)
	if !entering {
		return ast.WalkContinue, nil
	}
	dest := util.URLEscape(n.URL(source), false)
	if n.AutoLinkType == ast.AutoLinkEmail && !bytes.HasPrefix(bytes.ToLower(dest), []byte("mailto:")) {
		dest = append([]byte("mailto:"), dest...)
	}
	opened := islandOpenLink(w, dest, nil)
	w.Write(util.EscapeHTML(n.Label(source)))
	if opened {
		w.WriteString("</a>")
	}
	return ast.WalkContinue, nil
}

func (r islandRenderer) renderImage(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*ast.Image)
	dest := util.URLEscape(n.Destination, true)
	if r.inline || !isInlineImage(dest) {
		if !entering {
			if islandLinkOf(dest) != islandLinkNone {
				w.WriteString("</a>")
			}
			return ast.WalkContinue, nil
		}
		islandOpenLink(w, dest, n.Title)
		if n.FirstChild() == nil {
			w.Write(util.EscapeHTML(n.Destination))
		}
		return ast.WalkContinue, nil
	}
	return renderImage(w, source, node, entering)
}

func (r islandRenderer) renderParagraph(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering && node.PreviousSibling() != nil {
		w.WriteString("<br>\n")
	}
	return ast.WalkContinue, nil
}

func (r islandRenderer) renderCodeBlock(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		w.WriteString("</code></pre>\n")
		return ast.WalkContinue, nil
	}
	w.WriteString(`<pre tabindex="0"><code`)
	if n, ok := node.(*ast.FencedCodeBlock); ok && n.Language(source) != nil {
		w.WriteString(` class="language-`)
		gmhtml.DefaultWriter.Write(w, n.Language(source))
		w.WriteByte('"')
	}
	w.WriteByte('>')
	lines := node.Lines()
	for i := range lines.Len() {
		line := lines.At(i)
		gmhtml.DefaultWriter.RawWrite(w, line.Value(source))
	}
	return ast.WalkContinue, nil
}

func (r islandRenderer) renderTable(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		w.WriteString("</table>\n</div>\n")
		return ast.WalkContinue, nil
	}
	var titles []string
	if head, ok := node.FirstChild().(*east.TableHeader); ok {
		for cell := head.FirstChild(); cell != nil; cell = cell.NextSibling() {
			var b bytes.Buffer
			ast.Walk(cell, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
				switch t := n.(type) {
				case *ast.Text:
					if entering {
						b.Write(t.Value(source))
					}
				case *ast.String:
					if entering {
						b.Write(t.Value)
					}
				}
				return ast.WalkContinue, nil
			})
			if s := strings.Join(strings.Fields(b.String()), " "); s != "" {
				titles = append(titles, s)
			}
		}
	}
	label := "Table"
	if len(titles) > 0 {
		label = "Table: " + strings.Join(titles, ", ")
	}
	w.WriteString(`<div role="region" tabindex="0" aria-label="`)
	w.Write(util.EscapeHTML([]byte(label)))
	w.WriteString("\">\n<table>\n")
	return ast.WalkContinue, nil
}
