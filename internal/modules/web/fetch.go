package web

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/netip"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/doors-dev/gox"
	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"
	"neui.dev/internal/common"
)

const (
	fetchMaxLines = 2000
	fetchLineMax  = 2000
	fetchByteCap  = 50 << 10
	fetchAttrMax  = 200
)

var fetchedTag = regexp.MustCompile(`(?i)</?fetched`)

const fetchDesc = `Fetch one http or https URL with a GET and return its content inside a <fetched> element whose attributes are url, status, contentType, title when the document had one, and fetchedAt; with saveAs the element is empty and carries savedAs and bytes instead. url is where the content actually came from after redirects — resolve relative links against it, and read it to notice a consent wall or a login page. fetchedAt is when the response was read, which on a paged call is the original fetch and not this call. Pages are converted to markdown unless format says otherwise; text and html are for when the markdown loses something you need. Navigation, sidebars and comments are dropped unless full is set. Long pages are paged by line, unnumbered: pass offset to continue from where the previous call stopped, which reads the same fetched copy rather than requesting the page again. saveAs streams the raw bytes into the workspace instead of converting them, which is how a PDF or an image is fetched.`

type FetchArg struct {
	URL     string `json:"url" required:"true" description:"absolute http or https URL, credentials not allowed in it"`
	Format  string `json:"format" enum:"markdown,text,html" description:"markdown (default) converts the document, text strips it to prose, html returns the markup re-serialised, with scripts, styles, embeds, form controls, meta, links and comments removed; unless full is set it is the same main content the other formats pick, without navigation, headers, footers and sidebars"`
	Full    bool   `json:"full" description:"the page as a whole, including navigation, sidebars and comments; use it when the default came back missing something you saw referenced"`
	Offset  uint   `json:"offset" description:"1-based line of the converted content to start from, for continuing a long page"`
	Limit   uint   `json:"limit" description:"max lines to return, default 2000"`
	Timeout uint   `json:"timeoutMs" description:"how long to wait for the whole exchange, default 30000, at most 120000"`
	SaveAs  string `json:"saveAs" description:"workspace path to stream the raw body to instead of converting it; the only way to fetch a binary, and no format applies"`
}

func (m *module) fetch(ctx context.Context, arg FetchArg) (string, error) {
	u, err := url.Parse(strings.TrimSpace(arg.URL))
	if err != nil || !u.IsAbs() || u.Host == "" {
		return "", errors.New("url must be an absolute http or https URL, e.g. https://example.com/page")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("web__fetch reads http and https URLs only, not %s", u.Scheme)
	}
	if u.User != nil {
		return "", errors.New("the url carries credentials before the @; remove them, web__fetch sends no authentication")
	}
	host := u.Hostname()
	if _, err := netip.ParseAddr(host); err != nil && !strings.Contains(host, ".") {
		return "", errors.New("web__fetch cannot fetch localhost or other hostnames without a dot")
	}
	format := arg.Format
	if format == "" {
		format = "markdown"
	}
	key := u.String() + "\x00" + format
	if arg.Full {
		key += "\x00full"
	}
	if arg.SaveAs == "" {
		m.mu.Lock()
		cached, ok := m.cache[key]
		if ok && time.Since(cached.fetchedAt) > m.conf.CacheTTL {
			for k, c := range m.cache {
				if c == cached {
					delete(m.cache, k)
				}
			}
			m.held -= int64(len(cached.text))
			ok = false
		}
		if ok {
			m.tick++
			cached.used = m.tick
		}
		m.mu.Unlock()
		if ok {
			return page(cached, arg.Offset, arg.Limit)
		}
	}
	timeout := m.conf.Timeout
	if arg.Timeout != 0 {
		timeout = min(max(time.Duration(arg.Timeout)*time.Millisecond, time.Second), m.conf.MaxTimeout)
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var resp *http.Response
	for agent := browserAgent; ; agent = m.conf.UserAgent {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("User-Agent", agent)
		req.Header.Set("Accept", acceptHeader)
		req.Header.Set("Accept-Language", acceptLanguage)
		resp, err = m.client.Do(req)
		if err != nil {
			if resp != nil {
				resp.Body.Close()
			}
			var wrapped *url.Error
			if errors.As(err, &wrapped) {
				err = wrapped.Err
			}
			var refused blocked
			if errors.As(err, &refused) {
				failed := host
				if wrapped != nil {
					if at, perr := url.Parse(wrapped.URL); perr == nil && at.Hostname() != "" {
						failed = at.Hostname()
					}
				}
				return "", fmt.Errorf("%s resolves to %s, an address web__fetch does not connect to", failed, refused.addr)
			}
			if errors.Is(err, context.DeadlineExceeded) {
				return "", fmt.Errorf("fetching %s timed out after %s", u, timeout)
			}
			if errors.Is(err, context.Canceled) && ctx.Err() != nil {
				return "", context.Cause(ctx)
			}
			return "", err
		}
		if agent != m.conf.UserAgent && resp.StatusCode == http.StatusForbidden && strings.EqualFold(resp.Header.Get("cf-mitigated"), "challenge") {
			resp.Body.Close()
			continue
		}
		break
	}
	defer resp.Body.Close()
	final := resp.Request.URL.String()
	if resp.StatusCode >= 400 {
		head, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		excerpt := strings.Join(strings.Fields(string(head)), " ")
		if runes := []rune(excerpt); len(runes) > 512 {
			excerpt = string(runes[:512])
		}
		reason := strings.TrimSpace(strings.TrimPrefix(resp.Status, strconv.Itoa(resp.StatusCode)))
		if reason == "" {
			reason = http.StatusText(resp.StatusCode)
		}
		return "", fmt.Errorf("%s answered %d %s: %s", final, resp.StatusCode, reason, excerpt)
	}
	e := &entry{url: final, contentType: resp.Header.Get("Content-Type"), status: resp.StatusCode, fetchedAt: time.Now()}
	if runes := []rune(e.contentType); len(runes) > fetchAttrMax {
		e.contentType = string(runes[:fetchAttrMax])
	}
	if arg.SaveAs != "" {
		fs := common.GetController(ctx).FS()
		if resp.ContentLength > m.conf.MaxBytes {
			return "", fmt.Errorf("%s is over the %d-byte cap and was not saved", final, m.conf.MaxBytes)
		}
		partial := path.Join(path.Dir(arg.SaveAs), "."+path.Base(arg.SaveAs)+"."+rand.Text()+".part")
		w, err := fs.Create(partial)
		if err != nil {
			return "", err
		}
		n, err := io.Copy(w, io.LimitReader(resp.Body, m.conf.MaxBytes+1))
		if cerr := w.Close(); err == nil {
			err = cerr
		}
		if err == nil && n > m.conf.MaxBytes {
			err = fmt.Errorf("%s is over the %d-byte cap and was not saved", final, m.conf.MaxBytes)
		}
		if err == nil {
			err = fs.Rename(partial, arg.SaveAs)
		}
		if err != nil {
			fs.Remove(context.WithoutCancel(ctx), partial)
			return "", err
		}
		el := gox.Elem(func(cur gox.Cursor) error {
			cur.InitVoid("fetched")
			cur.Set("url", e.url)
			cur.Set("status", e.status)
			cur.Set("contentType", e.contentType)
			cur.Set("fetchedAt", e.fetchedAt.Format(layout))
			cur.Set("savedAs", arg.SaveAs)
			cur.Set("bytes", n)
			cur.Submit()
			return nil
		})
		b := strings.Builder{}
		el.Render(context.Background(), &b)
		return b.String(), nil
	}
	if resp.ContentLength > m.conf.MaxBytes {
		return "", fmt.Errorf("%s is over the %d-byte cap", final, m.conf.MaxBytes)
	}
	media, _, err := mime.ParseMediaType(e.contentType)
	if err != nil {
		media = strings.ToLower(strings.TrimSpace(strings.Split(e.contentType, ";")[0]))
	}
	markup := media == "text/html" || media == "application/xhtml+xml"
	textual := strings.HasPrefix(media, "text/") ||
		media == "application/json" || media == "application/xml" ||
		media == "application/javascript" || media == "application/x-javascript" ||
		strings.HasSuffix(media, "+json") || strings.HasSuffix(media, "+xml")
	if !markup && !textual && media != "" {
		return "", fmt.Errorf("%s is %s, which web__fetch does not convert; pass saveAs to keep the bytes in the workspace and read them with the files tools", final, media)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, m.conf.MaxBytes+1))
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return "", fmt.Errorf("fetching %s timed out after %s", u, timeout)
		}
		if errors.Is(err, context.Canceled) && ctx.Err() != nil {
			return "", context.Cause(ctx)
		}
		return "", err
	}
	if int64(len(raw)) > m.conf.MaxBytes {
		return "", fmt.Errorf("%s is over the %d-byte cap", final, m.conf.MaxBytes)
	}
	if media == "" {
		probe := raw
		if len(probe) > 512 {
			probe = probe[:512]
		}
		if bytes.IndexByte(probe, 0) >= 0 {
			return "", fmt.Errorf("%s looks binary and declares no content type; pass saveAs to keep the bytes in the workspace", final)
		}
	}
	decoded := raw
	if enc, _, certain := charset.DetermineEncoding(raw, e.contentType); enc != nil && (certain || !utf8.Valid(raw)) {
		if out, derr := enc.NewDecoder().Bytes(raw); derr == nil {
			decoded = out
		}
	}
	if !markup {
		e.text = string(decoded)
	} else {
		doc, err := html.Parse(bytes.NewReader(decoded))
		if err != nil {
			return "", err
		}
		e.title = docTitle(doc)
		if runes := []rune(e.title); len(runes) > fetchAttrMax {
			e.title = string(runes[:fetchAttrMax])
		}
		prune(doc, false)
		node := doc
		if !arg.Full {
			node = readable(doc, resp.Request.URL)
		}
		switch format {
		case "html":
			b := strings.Builder{}
			if err := html.Render(&b, node); err != nil {
				return "", err
			}
			e.text = b.String()
		case "text":
			b := bytes.Buffer{}
			plain(node, &b, false)
			e.text = strings.TrimSpace(b.String())
		default:
			out, cerr := m.conv.ConvertNode(node, converter.WithDomain(final), converter.WithContext(ctx))
			if cerr != nil {
				return "", cerr
			}
			e.text = strings.TrimSpace(string(out))
		}
	}
	size := int64(len(e.text))
	if size <= m.conf.CacheBytes {
		alias := final + "\x00" + format
		if arg.Full {
			alias += "\x00full"
		}
		m.mu.Lock()
		for _, k := range [2]string{key, alias} {
			old, ok := m.cache[k]
			if !ok {
				continue
			}
			for ck, c := range m.cache {
				if c == old {
					delete(m.cache, ck)
				}
			}
			m.held -= int64(len(old.text))
		}
		for m.held+size > m.conf.CacheBytes {
			var victim *entry
			for _, c := range m.cache {
				if victim == nil || c.used < victim.used {
					victim = c
				}
			}
			if victim == nil {
				break
			}
			for k, c := range m.cache {
				if c == victim {
					delete(m.cache, k)
				}
			}
			m.held -= int64(len(victim.text))
		}
		m.tick++
		e.used = m.tick
		m.cache[key] = e
		m.cache[alias] = e
		m.held += size
		m.mu.Unlock()
	}
	return page(e, arg.Offset, arg.Limit)
}

func page(e *entry, offset uint, limit uint) (string, error) {
	start := max(offset, 1)
	if limit == 0 {
		limit = fetchMaxLines
	}
	var b strings.Builder
	var total, shown, last uint
	budget := fetchByteCap
	capped := false
	cut := false
	for rest := e.text; rest != ""; {
		line, tail, found := strings.Cut(rest, "\n")
		rest = tail
		if !found {
			rest = ""
		}
		total++
		if total < start || shown == limit || capped {
			continue
		}
		line = strings.TrimSuffix(line, "\r")
		runes := 0
		for i := range line {
			if runes == fetchLineMax {
				line = line[:i] + "... (line truncated)"
				cut = true
				break
			}
			runes++
		}
		b.WriteString(line)
		b.WriteByte('\n')
		budget -= len(line)
		if budget <= 0 {
			capped = true
		}
		shown++
		last = total
	}
	switch {
	case total == 0:
		b.WriteString("(empty page)")
	case start > total:
		return "", fmt.Errorf("offset %d is beyond the end of the page, %d lines", start, total)
	default:
		if cut {
			fmt.Fprintf(&b, "(lines longer than %d characters were cut; what was cut cannot be reached with offset)\n", fetchLineMax)
		}
		if last == total {
			fmt.Fprintf(&b, "(end of page, %d lines)", total)
		} else {
			fmt.Fprintf(&b, "(showing lines %d-%d of %d, continue with offset=%d)", start, last, total, last+1)
		}
	}
	el := gox.Elem(func(cur gox.Cursor) error {
		cur.Init("fetched")
		cur.Set("url", e.url)
		cur.Set("status", e.status)
		cur.Set("contentType", e.contentType)
		if e.title != "" {
			cur.Set("title", e.title)
		}
		cur.Set("fetchedAt", e.fetchedAt.Format(layout))
		cur.Submit()
		cur.Raw(fetchedTag.ReplaceAllStringFunc("\n"+b.String()+"\n", func(tag string) string {
			return "&lt;" + tag[1:]
		}))
		cur.Close()
		return nil
	})
	out := strings.Builder{}
	if err := el.Render(context.Background(), &out); err != nil {
		return "", err
	}
	return out.String(), nil
}
