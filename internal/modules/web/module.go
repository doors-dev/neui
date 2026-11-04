package web

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"sync"
	"syscall"
	"time"

	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/table"
	"neui.dev/internal/common"
	"neui.dev/internal/tools"
)

const layout = "2006-01-02T15:04:05.000Z07:00"
const version = "0.1"
const honestAgent = "neui/" + version + " (+https://neui.dev)"
const browserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36"
const acceptHeader = "text/markdown, text/html;q=0.9, text/plain;q=0.8, */*;q=0.5"
const acceptLanguage = "en-US,en;q=0.9"
const maxRedirects = 10

const fetchModuleDesc = `The web, read-only. web__fetch gets one http or https URL and returns it as markdown (default), plain text or raw html. Nothing runs JavaScript, so a page that renders client-side comes back as an empty shell — fetch its API or a server-rendered URL instead. A long page arrives in line ranges: the content ends with a note naming the lines shown and the offset to continue with, and continuing costs no second request for about 15 minutes. Only text is returned; a PDF, an image or any other binary has to be fetched with saveAs into the workspace and read from there with the files tools.`

const searchModuleDesc = ` web__search asks a search backend for pages worth fetching, and returns links with extracts, not whole pages.`

var reserved = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("fec0::/10"),
}

type Config struct {
	HTTPClient  *http.Client
	Search      SearchBackend
	UserAgent   string
	MaxBytes    int64
	Timeout     time.Duration
	MaxTimeout  time.Duration
	CacheTTL    time.Duration
	CacheBytes  int64
	SearchChars int
}

func (c *Config) initDefaults() {
	if c.UserAgent == "" {
		c.UserAgent = honestAgent
	}
	if c.MaxBytes <= 0 {
		c.MaxBytes = 5 << 20
	}
	if c.Timeout <= 0 {
		c.Timeout = 30 * time.Second
	}
	if c.MaxTimeout <= 0 {
		c.MaxTimeout = 120 * time.Second
	}
	if c.CacheTTL <= 0 {
		c.CacheTTL = 15 * time.Minute
	}
	if c.CacheBytes <= 0 {
		c.CacheBytes = 8 << 20
	}
	if c.SearchChars <= 0 {
		c.SearchChars = 10000
	}
}

func Module(c Config) common.ModuleFactory {
	c.initDefaults()
	client := c.HTTPClient
	if client == nil {
		dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
		dialer.Control = func(_ string, address string, _ syscall.RawConn) error {
			ap, err := netip.ParseAddrPort(address)
			if err != nil {
				return blocked{addr: address}
			}
			a := ap.Addr().Unmap()
			if a.IsLoopback() || a.IsPrivate() || a.IsLinkLocalUnicast() || a.IsLinkLocalMulticast() || a.IsInterfaceLocalMulticast() || a.IsMulticast() || a.IsUnspecified() {
				return blocked{addr: a.String()}
			}
			for _, prefix := range reserved {
				if prefix.Contains(a) {
					return blocked{addr: a.String()}
				}
			}
			return nil
		}
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.DialContext = dialer.DialContext
		client = &http.Client{
			Transport: transport,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= maxRedirects {
					return redirected{text: via[0].URL.String() + " redirected more than 10 times"}
				}
				if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
					return redirected{text: via[len(via)-1].URL.String() + " redirected to " + req.URL.String() + ", which web__fetch cannot follow"}
				}
				req.URL.User = nil
				return nil
			},
		}
	}
	backend := c.Search
	desc := fetchModuleDesc
	if backend != nil {
		desc = fetchModuleDesc + searchModuleDesc
	}
	return func(context.Context) common.Module {
		m := tools.NewModule("web", desc)
		w := &module{
			conf:    c,
			client:  client,
			backend: backend,
			conv: converter.NewConverter(converter.WithPlugins(
				base.NewBasePlugin(),
				commonmark.NewCommonmarkPlugin(),
				table.NewTablePlugin(),
			)),
			cache: make(map[string]*entry),
		}
		if _, err := m.Add("fetch", fetchDesc, w.fetch); err != nil {
			panic(err)
		}
		if backend != nil {
			if _, err := m.Add("search", searchDesc, w.search); err != nil {
				panic(err)
			}
		}
		return m
	}
}

type module struct {
	conf    Config
	client  *http.Client
	backend SearchBackend
	conv    *converter.Converter
	mu      sync.Mutex
	cache   map[string]*entry
	held    int64
	tick    int64
}

type entry struct {
	url         string
	contentType string
	title       string
	text        string
	status      int
	fetchedAt   time.Time
	used        int64
}

type blocked struct {
	addr string
}

func (b blocked) Error() string {
	return "connections to " + b.addr + " are refused"
}

type redirected struct {
	text string
}

func (r redirected) Error() string {
	return r.text
}
