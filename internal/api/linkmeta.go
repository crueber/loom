// Server-side link metadata (link titles).
//
// GET /api/linkmeta?url=... fetches a public page and returns its
// <title> (or og:title) so a newly added link shows the page name
// instead of its raw URL. The fetch is SSRF-guarded: http/https only,
// private/loopback/link-local addresses blocked on every dial and
// redirect, short timeouts, and a capped read. Failures are soft — the
// handler answers 200 with an empty title and the client keeps the URL.
package api

import (
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	maxLinkMetaBytes = 512 * 1024
	maxLinkTitleLen  = 200
	maxLinkMetaURL   = 2048
	linkMetaTimeout  = 6 * time.Second
)

var (
	reHTMLTitle = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	reOGTitle   = regexp.MustCompile(`(?is)<meta[^>]+property\s*=\s*["']og:title["'][^>]*>`)
	reOGContent = regexp.MustCompile(`(?is)content\s*=\s*["']([^"']*)["']`)
	reTag       = regexp.MustCompile(`(?is)<[^>]*>`)
)

// FetchLinkTitle fetches raw and returns a page title. Injectable so
// tests never hit the network.
var FetchLinkTitle = fetchLinkTitle

// linkMeta serves link-title lookups for the add-link composer.
func (h *Handler) linkMeta(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	// The endpoint fetches remote URLs, so it follows the same gate as
	// writes: open when auth is off, session-gated when auth is on.
	if s := h.authSettings(); s.Enabled && h.CurrentUser(r) == nil {
		writeErr(w, http.StatusUnauthorized, "login required")
		return
	}
	raw := strings.TrimSpace(r.URL.Query().Get("url"))
	if raw == "" || len(raw) > maxLinkMetaURL {
		writeErr(w, http.StatusBadRequest, "invalid url")
		return
	}
	u, err := url.Parse(raw)
	if err != nil || checkLinkURL(u) != nil {
		writeErr(w, http.StatusBadRequest, "invalid url")
		return
	}
	title, err := FetchLinkTitle(raw)
	if err != nil {
		title = ""
	}
	writeJSON(w, http.StatusOK, map[string]string{"title": title})
}

// checkLinkURL accepts only public http(s) targets. It rejects the
// obvious local names/literal IPs early; the dialer and redirect hook
// enforce the same rule for every resolved address.
func checkLinkURL(u *url.URL) error {
	if u == nil {
		return errors.New("missing url")
	}
	if s := strings.ToLower(u.Scheme); s != "http" && s != "https" {
		return errors.New("unsupported scheme")
	}
	if u.User != nil {
		return errors.New("userinfo not allowed")
	}
	h := strings.ToLower(u.Hostname())
	if h == "" {
		return errors.New("missing host")
	}
	if h == "localhost" || strings.HasSuffix(h, ".localhost") || strings.HasSuffix(h, ".local") {
		return errors.New("local host")
	}
	if ip := net.ParseIP(h); ip != nil && blockedIP(ip) {
		return errors.New("private address")
	}
	return nil
}

// blockedIP reports addresses a link-title fetch may never dial.
func blockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() {
		return true
	}
	if v4 := ip.To4(); v4 != nil {
		switch {
		case v4[0] == 0:
			return true
		case v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127: // CGNAT
			return true
		case v4[0] == 192 && v4[1] == 0 && v4[2] == 0: // IETF assignments
			return true
		case v4[0] == 198 && (v4[1] == 18 || v4[1] == 19): // benchmarking
			return true
		}
	}
	return false
}

// safeDialContext resolves the host once and dials only vetted IPs, so
// a DNS name that later rebinds to a private address cannot be used to
// reach the loopback/LAN.
func safeDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	var lastErr error = errors.New("no usable address")
	for _, ip := range ips {
		if blockedIP(ip.IP) {
			lastErr = errors.New("blocked address")
			continue
		}
		d := &net.Dialer{Timeout: 4 * time.Second}
		conn, derr := d.DialContext(ctx, network, net.JoinHostPort(ip.IP.String(), port))
		if derr == nil {
			return conn, nil
		}
		lastErr = derr
	}
	return nil, lastErr
}

var linkMetaClient = &http.Client{
	Timeout: linkMetaTimeout,
	Transport: &http.Transport{
		Proxy:                 nil,
		DialContext:           safeDialContext,
		TLSHandshakeTimeout:   4 * time.Second,
		ResponseHeaderTimeout: 5 * time.Second,
		DisableKeepAlives:     true,
	},
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("too many redirects")
		}
		return checkLinkURL(req.URL)
	},
}

func fetchLinkTitle(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if err := checkLinkURL(u); err != nil {
		return "", err
	}
	return fetchLinkTitleWith(linkMetaClient, raw)
}

// fetchLinkTitleWith performs the request with the given client. Split
// out so tests can exercise the HTML/status/content-type path without
// dialing (and without the loopback guard rejecting the test server).
func fetchLinkTitleWith(client *http.Client, raw string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, raw, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; LoomBot/1.0)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml;q=0.9,*/*;q=0.1")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("upstream %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "" &&
		!strings.HasPrefix(ct, "text/html") && !strings.HasPrefix(ct, "application/xhtml") {
		return "", errors.New("not html")
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxLinkMetaBytes))
	if err != nil {
		return "", err
	}
	return parseHTMLTitle(string(body)), nil
}

// parseHTMLTitle extracts the first <title>, falling back to og:title,
// then normalizes it: tags dropped, entities decoded, whitespace
// collapsed, control characters stripped, and length capped.
func parseHTMLTitle(doc string) string {
	var raw string
	if m := reHTMLTitle.FindStringSubmatch(doc); m != nil {
		raw = m[1]
	} else if m := reOGTitle.FindString(doc); m != "" {
		if c := reOGContent.FindStringSubmatch(m); c != nil {
			raw = c[1]
		}
	}
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	raw = reTag.ReplaceAllString(raw, " ")
	raw = html.UnescapeString(raw)
	raw = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' {
			return ' '
		}
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, raw)
	raw = strings.Join(strings.Fields(raw), " ")
	if utf8.RuneCountInString(raw) > maxLinkTitleLen {
		raw = strings.TrimSpace(string([]rune(raw)[:maxLinkTitleLen]))
	}
	return raw
}
