// Package rest provides bounded, credential-safe GET requests for CI providers.
package rest

import (
	"ciview/internal/ci"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	maxResponseBytes = 16 << 20
	maxCacheBytes    = 8 << 20
	maxCacheEntries  = 128
)

type cachedResponse struct {
	body   []byte
	header http.Header
	url    url.URL
	etag   string
	bytes  int
}

type Transport struct {
	Base              *url.URL
	Source, Provider  string
	client            *http.Client
	token, authScheme string
	pageMu            sync.Mutex
	pageSizes         map[string]int
	cacheMu           sync.Mutex
	responses         map[string]cachedResponse
	cacheBytes        int
}

type Page struct {
	Header http.Header
	URL    *url.URL
}

func New(source, provider, base, token, authScheme string, client *http.Client) (*Transport, error) {
	u, e := url.Parse(base)
	if e != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.RawPath != "" || path.Clean(u.Path) != strings.TrimSuffix(u.Path, "/") && u.Path != "" && u.Path != "/" {
		return nil, Error("configuration", "Invalid provider base URL")
	}
	u.Path = strings.TrimSuffix(u.Path, "/")
	if strings.TrimSpace(source) == "" {
		return nil, Error("configuration", "Provider source name is required")
	}
	var owned http.Client
	if client != nil {
		owned = *client
	}
	if owned.Timeout == 0 {
		owned.Timeout = 20 * time.Second
	}
	t := &Transport{Base: u, Source: source, Provider: provider, client: &owned, token: token, authScheme: authScheme, pageSizes: map[string]int{}, responses: map[string]cachedResponse{}}
	original := owned.CheckRedirect
	owned.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if !t.safe(req.URL) {
			return Error("unsafe_url", "Provider redirect was rejected")
		}
		if len(via) >= 10 {
			return Error("unavailable", "Too many provider redirects")
		}
		if original != nil {
			return original(req, via)
		}
		return nil
	}
	return t, nil
}

func Error(kind, message string) *ci.ProviderError {
	return &ci.ProviderError{Kind: kind, Message: message}
}

func (t *Transport) safe(u *url.URL) bool {
	if u == nil || !strings.EqualFold(u.Scheme, t.Base.Scheme) || !strings.EqualFold(u.Host, t.Base.Host) || u.User != nil || u.Fragment != "" {
		return false
	}
	// Reject encoded separators and traversal before path-prefix checking.
	decoded, e := url.PathUnescape(u.EscapedPath())
	if e != nil || strings.Contains(decoded, "\\") || path.Clean(decoded) != decoded {
		return false
	}
	return t.Base.Path == "" || decoded == t.Base.Path || strings.HasPrefix(decoded, t.Base.Path+"/")
}

func (t *Transport) Endpoint(p string, q url.Values) (string, error) {
	if !strings.HasPrefix(p, "/") || strings.Contains(p, "\\") || path.Clean(p) != p {
		return "", Error("invalid_data", "Invalid provider resource path")
	}
	u := *t.Base
	u.Path = t.Base.Path + p
	u.RawPath = ""
	u.RawQuery = q.Encode()
	if !t.safe(&u) {
		return "", Error("unsafe_url", "Provider request URL was rejected")
	}
	return u.String(), nil
}

func Segment(s string) (string, error) {
	if s == "" || s == "." || s == ".." || strings.ContainsAny(s, "/\\?#%\x00\r\n") {
		return "", Error("invalid_data", "Invalid provider resource identity")
	}
	return s, nil
}

func RepoPath(r ci.Repository) (string, error) {
	owner, e := Segment(r.Owner)
	if e != nil {
		return "", e
	}
	name, e := Segment(r.Name)
	if e != nil {
		return "", e
	}
	return "/repos/" + owner + "/" + name, nil
}

func (t *Transport) Get(ctx context.Context, target string, dst any) (Page, error) {
	u, e := url.Parse(target)
	if e != nil || !t.safe(u) {
		return Page{}, Error("unsafe_url", "Provider request URL was rejected")
	}
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if e != nil {
		return Page{}, Error("invalid_data", "Invalid provider request")
	}
	req.Header.Set("Accept", "application/json")
	if t.Provider == "github" {
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2026-03-10")
	}
	if t.token != "" {
		req.Header.Set("Authorization", t.authScheme+" "+t.token)
	}
	cacheKey := u.String()
	t.cacheMu.Lock()
	cached, cacheHit := t.responses[cacheKey]
	t.cacheMu.Unlock()
	if cacheHit {
		req.Header.Set("If-None-Match", cached.etag)
	}
	response, e := t.client.Do(req)
	if e != nil {
		if response != nil && response.Body != nil {
			response.Body.Close()
		}
		if ctx.Err() != nil {
			return Page{}, ctx.Err()
		}
		var pe *ci.ProviderError
		if errors.As(e, &pe) {
			return Page{}, pe
		}
		return Page{}, Error("unavailable", "Provider request failed")
	}
	defer response.Body.Close()
	var body []byte
	var p Page
	if response.StatusCode == http.StatusNotModified {
		if !cacheHit {
			return Page{}, Error("invalid_data", "Provider returned an uncached conditional response")
		}
		body = cached.body
		header := cached.header.Clone()
		for key, values := range response.Header {
			header[key] = append([]string(nil), values...)
		}
		finalURL := cached.url
		p = Page{Header: header, URL: &finalURL}
	} else {
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			return Page{}, statusError(response)
		}
		body, e = io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
		if e != nil || len(body) > maxResponseBytes {
			return Page{}, Error("invalid_data", "Provider response could not be read")
		}
		p = Page{Header: response.Header, URL: response.Request.URL}
	}
	if strings.TrimSpace(string(body)) == "null" {
		return Page{}, Error("invalid_data", "Provider response was invalid")
	}
	if e = json.Unmarshal(body, dst); e != nil {
		return Page{}, Error("invalid_data", "Provider response was invalid")
	}
	if _, e = t.NextLink(p); e != nil {
		return Page{}, e
	}
	if response.StatusCode == http.StatusOK {
		t.cacheResponse(cacheKey, body, p)
	}

	return p, nil
}

// Cache only validated successful representations. Entries are immutable and
// local to this transport's source/credential. No lock spans a network request.
func (t *Transport) cacheResponse(key string, body []byte, p Page) {
	etag := p.Header.Get("ETag")
	cost := len(key) + len(body) + len(p.URL.String())
	for name, values := range p.Header {
		cost += len(name)
		for _, value := range values {
			cost += len(value)
		}
	}
	t.cacheMu.Lock()
	defer t.cacheMu.Unlock()
	if prior, ok := t.responses[key]; ok {
		t.cacheBytes -= prior.bytes
		delete(t.responses, key)
	}
	if etag == "" || cost > maxCacheBytes {
		return
	}
	if len(t.responses) >= maxCacheEntries || t.cacheBytes+cost > maxCacheBytes {
		clear(t.responses)
		t.cacheBytes = 0
	}
	t.responses[key] = cachedResponse{body: body, header: p.Header.Clone(), url: *p.URL, etag: etag, bytes: cost}
	t.cacheBytes += cost
}

func statusError(r *http.Response) *ci.ProviderError {
	kind, message := "unavailable", "Provider is unavailable"
	switch r.StatusCode {
	case 401:
		kind, message = "authentication", "Provider authentication failed"
	case 403:
		kind, message = "permission", "Provider access was denied"
	case 404:
		kind, message = "not_found", "Repository or Actions resource is unavailable"
	case 429:
		kind, message = "rate_limit", "Provider rate limit reached"
	}
	retry := RetryAt(r.Header, time.Now().UTC())
	if r.StatusCode == 403 && (retry != nil || r.Header.Get("X-RateLimit-Remaining") == "0") {
		kind, message = "rate_limit", "Provider rate limit reached"
	}
	return &ci.ProviderError{Kind: kind, Message: message, RetryAt: retry}
}

func RetryAt(h http.Header, now time.Time) *time.Time {
	var retry time.Time
	if raw := h.Get("Retry-After"); raw != "" {
		if seconds, e := strconv.ParseInt(raw, 10, 32); e == nil && seconds >= 0 {
			retry = now.Add(time.Duration(seconds) * time.Second)
		} else if date, e := http.ParseTime(raw); e == nil {
			retry = date.UTC()
		}
	}
	if h.Get("X-RateLimit-Remaining") == "0" {
		if epoch, e := strconv.ParseInt(h.Get("X-RateLimit-Reset"), 10, 64); e == nil {
			reset := time.Unix(epoch, 0).UTC()
			if reset.After(retry) {
				retry = reset
			}
		}
	}
	if retry.IsZero() {
		return nil
	}
	if retry.Before(now) {
		retry = now
	}
	return &retry
}

func (t *Transport) NextLink(p Page) (string, error) {
	for _, part := range strings.Split(p.Header.Get("Link"), ",") {
		pieces := strings.Split(part, ";")
		if len(pieces) < 2 {
			continue
		}
		next := false
		for _, attr := range pieces[1:] {
			attr = strings.TrimSpace(attr)
			if strings.HasPrefix(attr, "rel=") {
				for _, rel := range strings.Fields(strings.Trim(strings.TrimPrefix(attr, "rel="), "\"")) {
					if rel == "next" {
						next = true
					}
				}
			}
		}
		if !next {
			continue
		}
		target := strings.TrimSpace(pieces[0])
		if !strings.HasPrefix(target, "<") || !strings.HasSuffix(target, ">") {
			return "", Error("invalid_data", "Provider pagination was invalid")
		}
		u, e := url.Parse(target[1 : len(target)-1])
		if e != nil {
			return "", Error("unsafe_url", "Provider pagination URL was rejected")
		}
		u = p.URL.ResolveReference(u)
		if !t.safe(u) {
			return "", Error("unsafe_url", "Provider pagination URL was rejected")
		}
		return u.String(), nil
	}
	return "", nil
}

// HasNext uses Link when available. Numbered APIs may cap the requested
// limit; a non-final first page reveals that effective size for later offsets.
func (t *Transport) HasNext(p Page, page, size, count, total int) (bool, error) {
	if n, e := strconv.Atoi(p.Header.Get("X-Total-Count")); e == nil && n >= 0 {
		total = n
	}
	effective, known := size, page == 1
	key := ""
	if p.URL != nil {
		u := *p.URL
		q := u.Query()
		q.Del("page")
		u.RawQuery = q.Encode()
		key = u.String()
	}
	t.pageMu.Lock()
	if page == 1 {
		if count > 0 && count < size && total > count {
			effective = count
		}
		if len(t.pageSizes) >= 2048 {
			clear(t.pageSizes)
		}
		t.pageSizes[key] = effective
	} else if cached, ok := t.pageSizes[key]; ok {
		effective, known = cached, true
	}
	t.pageMu.Unlock()
	next, e := t.NextLink(p)
	if e != nil {
		return false, e
	}
	if next != "" {
		return true, nil
	}
	if p.Header.Get("Link") != "" {
		return false, nil
	}
	if total >= 0 {
		if !known {
			// Without a first page the offset is ambiguous. Continue conservatively
			// rather than claiming complete coverage using the requested limit.
			return count > 0 && count < total, nil
		}
		offset := (page - 1) * effective
		if count == 0 && offset < total {
			return false, Error("invalid_data", "Provider page was incomplete")
		}
		return offset+count < total, nil
	}
	return count >= effective, nil
}

func Timestamp(raw string) *time.Time {
	v, e := time.Parse(time.RFC3339Nano, raw)
	if e != nil || v.IsZero() || !v.After(time.Unix(0, 0)) {
		return nil
	}
	v = v.UTC()
	return &v
}
func SafeURL(raw string) string {
	u, e := url.Parse(raw)
	if e != nil || u.Host == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return ""
	}
	return u.String()
}
func StableKey(parts ...string) ci.RepositoryKey {
	b, _ := json.Marshal(parts)
	sum := sha256.Sum256(b)
	return ci.RepositoryKey(hex.EncodeToString(sum[:]))
}
