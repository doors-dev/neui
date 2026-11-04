package web

import (
	"bytes"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	readability "codeberg.org/readeck/go-readability/v2"
	"golang.org/x/net/html"
)

var pruneTag = map[string]bool{
	"script": true, "style": true, "noscript": true, "iframe": true,
	"object": true, "embed": true, "svg": true, "meta": true, "link": true,
	"form": true,
}

var boilerTag = map[string]bool{
	"nav": true, "header": true, "footer": true, "aside": true,
}

var blockTag = map[string]bool{
	"address": true, "article": true, "aside": true, "blockquote": true,
	"body": true, "details": true, "div": true, "dd": true, "dl": true,
	"dt": true, "fieldset": true, "figcaption": true, "figure": true,
	"footer": true, "h1": true, "h2": true, "h3": true, "h4": true,
	"h5": true, "h6": true, "header": true, "hr": true, "li": true,
	"main": true, "nav": true, "ol": true, "p": true, "pre": true,
	"section": true, "summary": true, "table": true, "tbody": true,
	"td": true, "tfoot": true, "th": true, "thead": true, "tr": true,
	"ul": true,
}

func docTitle(n *html.Node) string {
	if n.Type == html.ElementNode && n.Data == "title" {
		var b strings.Builder
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.TextNode {
				b.WriteString(c.Data)
			}
		}
		return strings.Join(strings.Fields(b.String()), " ")
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if t := docTitle(c); t != "" {
			return t
		}
	}
	return ""
}

func prune(n *html.Node, full bool) {
	child := n.FirstChild
	for child != nil {
		next := child.NextSibling
		drop := child.Type == html.CommentNode
		if child.Type == html.ElementNode {
			drop = pruneTag[child.Data] || (!full && boilerTag[child.Data])
		}
		if drop {
			n.RemoveChild(child)
		} else {
			prune(child, full)
		}
		child = next
	}
}

func readable(doc *html.Node, page *url.URL) *html.Node {
	article, err := readability.FromDocument(doc, page)
	if err != nil || article.Node == nil {
		return doc
	}
	return article.Node
}

func plain(n *html.Node, b *bytes.Buffer, full bool) {
	switch n.Type {
	case html.CommentNode, html.DoctypeNode:
		return
	case html.TextNode:
		words := strings.Fields(n.Data)
		lead, _ := utf8.DecodeRuneInString(n.Data)
		if n.Data != "" && (len(words) == 0 || unicode.IsSpace(lead)) && b.Len() > 0 {
			if last := b.Bytes()[b.Len()-1]; last != ' ' && last != '\n' {
				b.WriteByte(' ')
			}
		}
		b.WriteString(strings.Join(words, " "))
		if tail, _ := utf8.DecodeLastRuneInString(n.Data); len(words) > 0 && unicode.IsSpace(tail) {
			b.WriteByte(' ')
		}
		return
	}
	if n.Type == html.ElementNode {
		if pruneTag[n.Data] || (!full && boilerTag[n.Data]) || n.Data == "head" {
			return
		}
		if n.Data == "br" {
			newline(b)
			return
		}
	}
	block := n.Type == html.ElementNode && blockTag[n.Data]
	if block {
		newline(b)
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		plain(c, b, full)
	}
	if block {
		newline(b)
	}
}

func newline(b *bytes.Buffer) {
	for b.Len() > 0 && b.Bytes()[b.Len()-1] == ' ' {
		b.Truncate(b.Len() - 1)
	}
	if b.Len() > 0 && b.Bytes()[b.Len()-1] != '\n' {
		b.WriteByte('\n')
	}
}
