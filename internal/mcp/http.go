package mcp

import (
	"fmt"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// ParseAllowedHosts accepts exact DNS names or IP addresses, never URLs or ports.
func ParseAllowedHosts(value string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	var out []string
	for _, part := range strings.Split(value, ",") {
		host := strings.ToLower(strings.TrimSpace(part))
		if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
			host = strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
		}
		if !validHost(host) {
			return nil, fmt.Errorf("MCP allowed hosts must contain exact hostnames or IP addresses")
		}
		out = append(out, host)
	}
	return out, nil
}
func validHost(host string) bool {
	if net.ParseIP(host) != nil {
		return true
	}
	if len(host) == 0 || len(host) > 253 {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, c := range label {
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
				return false
			}
		}
	}
	return true
}
func requestHost(authority string) (string, bool) {
	u, err := url.Parse("http://" + authority)
	if err != nil || u.Host != authority || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return "", false
	}
	host := strings.ToLower(u.Hostname())
	if !validHost(host) {
		return "", false
	}
	if port := u.Port(); port != "" {
		n, e := strconv.Atoi(port)
		if e != nil || n < 1 || n > 65535 {
			return "", false
		}
	} else if strings.HasSuffix(authority, ":") {
		return "", false
	}
	// IPv6 authorities must be bracketed.
	if strings.Contains(host, ":") && !strings.HasPrefix(authority, "[") {
		return "", false
	}
	return host, true
}
func protect(next http.Handler, hosts []string) http.Handler {
	allowed := map[string]bool{"localhost": true, "127.0.0.1": true, "::1": true}
	for _, host := range hosts {
		allowed[strings.ToLower(strings.Trim(host, "[]"))] = true
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		host, ok := requestHost(r.Host)
		if !ok || !allowed[host] {
			http.Error(w, "MCP host is not allowed", 403)
			return
		}
		if origins, exists := r.Header["Origin"]; exists {
			origin := ""
			if len(origins) == 1 {
				origin = origins[0]
			}
			u, err := url.Parse(origin)
			if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || !strings.EqualFold(u.Host, r.Host) || u.User != nil || u.Path != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.Contains(origin, "#") {
				http.Error(w, "MCP origin is not allowed", 403)
				return
			}
		}
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			http.Error(w, "Use Streamable HTTP POST for MCP", 405)
			return
		}
		next.ServeHTTP(w, r)
	})
}
func transport(server *sdk.Server, hosts []string) http.Handler {
	return protect(sdk.NewStreamableHTTPHandler(func(*http.Request) *sdk.Server { return server }, &sdk.StreamableHTTPOptions{Stateless: true, JSONResponse: true, DisableLocalhostProtection: true, PropagateRequestCancellation: true, MaxRequestBodyBytes: 1 << 20}), hosts)
}
