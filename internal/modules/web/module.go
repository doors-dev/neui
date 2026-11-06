package web

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"sync"
	"syscall"
	"time"

	"code.dny.dev/ssrf"
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

const fetchModuleDesc = `The web, read-only. web__fetch gets one http or https URL and returns it as markdown (default), plain text or html stripped of scripts, styles, embeds, form controls, meta and comments. Nothing runs JavaScript, so a page that renders client-side comes back as an empty shell — fetch its API or a server-rendered URL instead. A long page arrives in line ranges: the content ends with a note naming the lines shown and the offset to continue with, and continuing costs no second request for about 15 minutes. Only text is returned; a PDF, an image or any other binary has to be fetched with saveAs into the workspace and read from there with the files tools.`

const searchModuleDesc = ` web__search asks a search backend for pages worth fetching, and returns links with extracts, not whole pages.`

var guardian = ssrf.New(ssrf.WithAnyPort())

var nat64 = netip.MustParsePrefix("64:ff9b::/96")

var badPort = map[uint16]bool{
	0: true, 1: true, 7: true, 9: true, 11: true, 13: true, 15: true, 17: true,
	19: true, 20: true, 21: true, 22: true, 23: true, 25: true, 37: true,
	42: true, 43: true, 53: true, 69: true, 77: true, 79: true, 87: true,
	95: true, 101: true, 102: true, 103: true, 104: true, 109: true, 110: true,
	111: true, 113: true, 115: true, 117: true, 119: true, 123: true, 135: true,
	137: true, 139: true, 143: true, 161: true, 179: true, 389: true, 427: true,
	465: true, 512: true, 513: true, 514: true, 515: true, 526: true, 530: true,
	531: true, 532: true, 540: true, 548: true, 554: true, 556: true, 563: true,
	587: true, 601: true, 636: true, 989: true, 990: true, 993: true, 995: true,
	1719: true, 1720: true, 1723: true, 2049: true, 3659: true, 4045: true,
	4190: true, 5060: true, 5061: true, 6000: true, 6566: true, 6665: true,
	6666: true, 6667: true, 6668: true, 6669: true, 6679: true, 6697: true,
	10080: true,
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
		c.SearchChars = 2000
	}
}

func Module(c Config) common.ModuleFactory {
	c.initDefaults()
	client := c.HTTPClient
	if client == nil {
		client = guardedClient()
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

func guard(_ string, address string, _ syscall.RawConn) error {
	ap, err := netip.ParseAddrPort(address)
	if err != nil {
		return blocked{addr: address}
	}
	a := ap.Addr().Unmap()
	if nat64.Contains(a) {
		b := a.As16()
		a = netip.AddrFrom4([4]byte(b[12:]))
	}
	if badPort[ap.Port()] {
		return badPorted{port: ap.Port()}
	}
	if guardian.SafeAddrPort(netip.AddrPortFrom(a, ap.Port())) != nil {
		return blocked{addr: a.String()}
	}
	return nil
}

func guardedClient() *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second, Control: guard}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = dialer.DialContext
	return &http.Client{
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

type badPorted struct {
	port uint16
}

func (b badPorted) Error() string {
	return "connections to port " + strconv.Itoa(int(b.port)) + " are refused"
}

type redirected struct {
	text string
}

func (r redirected) Error() string {
	return r.text
}
