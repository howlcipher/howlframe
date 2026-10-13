package capability

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// FetchGrantPrefix scopes outbound HTTP (fetch, http_request) to one exact
// host and port. The coarse "network" grant never authorizes an outbound
// request: it covers serving (http_server, routes) and the model forms only.
//
// Syntax: network:fetch=<host>[:<port>]
//
//	host  a DNS name (matched exactly, case-insensitively; no wildcards,
//	      no suffix matching) or an IP literal ([v6] in brackets)
//	port  optional; when omitted only the scheme default (80 for http,
//	      443 for https) matches
//
// Every redirect hop is re-checked against the same allow-list, an IP
// literal matches only a grant that names that same IP, and a granted DNS
// name may not resolve to private, loopback, NAT64, link-local or unspecified
// addresses, except for the explicit localhost opt-out. Multicast is always
// refused. Proxies from the environment are ignored.
const FetchGrantPrefix = "network:fetch="

// FetchDenied is the typed detail of a refused outbound request. Required is
// the grant that would have authorized it (network:fetch=host:port); it never
// carries the path, query or userinfo of the URL.
type FetchDenied struct {
	Reason   string
	Required string
}

func (e *FetchDenied) Error() string { return e.Reason }

// fetchTarget is a normalized host:port pair.
type fetchTarget struct {
	host string // lower-case name, or canonical IP text (no brackets)
	port string // "" in a grant means scheme default
	ip   bool
}

func (t fetchTarget) String() string {
	h := t.host
	if strings.Contains(h, ":") {
		h = "[" + h + "]"
	}
	if t.port == "" {
		return h
	}
	return h + ":" + t.port
}

// looksNumeric reports a host made only of hex digits, dots and an x, which
// resolvers may read as a short or octal IPv4 form (127.1, 0x7f.1, 2130706433).
func looksNumeric(h string) bool {
	if h == "" {
		return false
	}
	last := h
	if i := strings.LastIndex(h, "."); i >= 0 {
		last = h[i+1:]
	}
	if last == "" {
		return true
	}
	if last[0] >= '0' && last[0] <= '9' {
		return true
	}
	return false
}

func validHostname(h string) bool {
	if len(h) == 0 || len(h) > 253 || strings.HasSuffix(h, ".") || strings.HasPrefix(h, ".") {
		return false
	}
	for _, label := range strings.Split(h, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, r := range label {
			if !(r == '-' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9') {
				return false
			}
		}
	}
	return true
}

// normalizeHost validates a host as written in a grant or a URL.
func normalizeHost(h string) (string, bool, error) {
	if strings.HasPrefix(h, "[") {
		if !strings.HasSuffix(h, "]") {
			return "", false, fmt.Errorf("bad IPv6 literal")
		}
		addr, err := netip.ParseAddr(h[1 : len(h)-1])
		if err != nil || !addr.Is6() || addr.Zone() != "" {
			return "", false, fmt.Errorf("bad IPv6 literal")
		}
		return canonicalIP(addr), true, nil
	}
	if addr, err := netip.ParseAddr(h); err == nil {
		if !addr.Is4() {
			return "", false, fmt.Errorf("IPv6 literals must be bracketed")
		}
		return canonicalIP(addr), true, nil
	}
	h = strings.ToLower(h)
	if strings.HasSuffix(h, ".") {
		return "", false, fmt.Errorf("invalid host name: trailing dot")
	}
	if looksNumeric(h) {
		return "", false, fmt.Errorf("ambiguous numeric host")
	}
	if !validHostname(h) {
		return "", false, fmt.Errorf("invalid host name")
	}
	return h, false, nil
}

func canonicalIP(a netip.Addr) string {
	if a.Is4In6() {
		a = a.Unmap()
	}
	return a.String()
}

func validPort(p string) bool {
	n, err := strconv.Atoi(p)
	return err == nil && n >= 1 && n <= 65535 && strconv.Itoa(n) == p
}

// splitHostPort splits "host", "host:port", "[v6]" or "[v6]:port".
func splitHostPort(s string) (string, string, error) {
	if strings.HasPrefix(s, "[") {
		end := strings.Index(s, "]")
		if end < 0 {
			return "", "", fmt.Errorf("bad IPv6 literal")
		}
		host, rest := s[:end+1], s[end+1:]
		if rest == "" {
			return host, "", nil
		}
		if !strings.HasPrefix(rest, ":") {
			return "", "", fmt.Errorf("bad port")
		}
		return host, rest[1:], nil
	}
	if i := strings.LastIndex(s, ":"); i >= 0 {
		return s[:i], s[i+1:], nil
	}
	return s, "", nil
}

func parseFetchSpec(spec string) (fetchTarget, error) {
	if spec == "" || strings.ContainsAny(spec, "/@?#*% \t\\") {
		return fetchTarget{}, fmt.Errorf("network:fetch grant must be host[:port] with no scheme, path, wildcard or userinfo")
	}
	h, p, err := splitHostPort(spec)
	if err != nil {
		return fetchTarget{}, fmt.Errorf("network:fetch grant: %v", err)
	}
	if p != "" && !validPort(p) {
		return fetchTarget{}, fmt.Errorf("network:fetch grant: invalid port")
	}
	if strings.Contains(spec, ":") && !strings.HasPrefix(spec, "[") && p == "" {
		return fetchTarget{}, fmt.Errorf("network:fetch grant: empty port")
	}
	host, ip, err := normalizeHost(h)
	if err != nil {
		return fetchTarget{}, fmt.Errorf("network:fetch grant: %v", err)
	}
	return fetchTarget{host: host, port: p, ip: ip}, nil
}

func parseFetchGrant(raw string) (Capability, bool, error) {
	spec, ok := strings.CutPrefix(raw, FetchGrantPrefix)
	if !ok {
		return None, false, nil
	}
	t, err := parseFetchSpec(spec)
	if err != nil {
		return None, true, err
	}
	return Capability(FetchGrantPrefix + t.String()), true, nil
}

// HasFetch reports whether any host-scoped fetch grant is present.
func (g Grants) HasFetch() bool {
	for _, grant := range g {
		if _, ok, err := parseFetchGrant(string(grant)); ok && err == nil {
			return true
		}
	}
	return false
}

func urlTarget(u *url.URL) (fetchTarget, error) {
	deny := func(reason string) (fetchTarget, error) {
		return fetchTarget{}, &FetchDenied{Reason: "network: " + reason, Required: FetchGrantPrefix + "<invalid-url>"}
	}
	if u == nil {
		return deny("fetch URL is invalid")
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return deny("fetch URL scheme must be http or https")
	}
	if u.Host == "" {
		return deny("fetch URL has no host")
	}
	h, p, err := splitHostPort(u.Host)
	if err != nil || h == "" {
		return deny("fetch URL host is invalid")
	}
	if p == "" {
		if strings.HasSuffix(u.Host, ":") {
			return deny("fetch URL port is invalid")
		}
		p = "80"
		if scheme == "https" {
			p = "443"
		}
	} else if !validPort(p) {
		return deny("fetch URL port is invalid")
	}
	host, ip, err := normalizeHost(h)
	if err != nil {
		return deny("fetch URL host is invalid (" + err.Error() + ")")
	}
	return fetchTarget{host: host, port: p, ip: ip}, nil
}

func defaultPort(scheme string) string {
	if strings.ToLower(scheme) == "https" {
		return "443"
	}
	return "80"
}

// AuthorizeFetch checks one outbound URL (an initial request or a redirect
// hop) against the network:fetch= allow-list. It returns the matched grant,
// or a *FetchDenied whose Required names the missing grant.
func (g Grants) AuthorizeFetch(u *url.URL) (string, error) {
	t, err := urlTarget(u)
	if err != nil {
		return "", err
	}
	for _, grant := range g {
		parsed, ok, err := parseFetchGrant(string(grant))
		if !ok || err != nil {
			continue
		}
		gt, _ := parseFetchSpec(strings.TrimPrefix(string(parsed), FetchGrantPrefix))
		if gt.host != t.host {
			continue
		}
		if gt.port == t.port || gt.port == "" && t.port == defaultPort(u.Scheme) {
			return string(parsed), nil
		}
	}
	return "", &FetchDenied{Reason: "network: fetch host is not in the allow-list", Required: FetchGrantPrefix + t.String()}
}

// forbiddenResolved reports addresses a hostname grant may not reach by
// default. Explicit IP-literal and localhost grants opt out, except multicast.
func forbiddenResolved(a netip.Addr) bool {
	a = a.Unmap()
	return ForbiddenDestination(a)
}

// checkDial vets the resolved address a connection is about to use. A DNS
// name may not land on a forbidden destination. Explicit literal and
// localhost grants opt out of private-address checks, but never multicast.
func checkDial(host string, literal bool, address string) error {
	ap, err := netip.ParseAddrPort(address)
	if err != nil {
		return &FetchDenied{Reason: "network: fetch dial address is invalid", Required: FetchGrantPrefix + host}
	}
	if ap.Addr().IsMulticast() || !literal && !strings.EqualFold(host, "localhost") && forbiddenResolved(ap.Addr()) {
		return &FetchDenied{Reason: "network: granted host resolved to a forbidden destination", Required: FetchGrantPrefix + canonicalIP(ap.Addr())}
	}
	return nil
}

// MaxFetchRedirects bounds redirect hops; each hop is re-authorized.
const MaxFetchRedirects = 10

// FetchClient returns an HTTP client that re-authorizes every redirect hop
// against g, ignores proxy settings, and vets the resolved destination at
// connection time. onHop is called for every
// redirect hop with the matched grant or the denial. A denial surfaces from
// Do as an error that wraps *FetchDenied.
func FetchClient(g Grants, onHop func(u *url.URL, grant string, err error)) *http.Client {
	dialer := &net.Dialer{Timeout: 30 * time.Second}
	transport := &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, _, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			literal := false
			if _, err := netip.ParseAddr(host); err == nil {
				literal = true
			}
			d := *dialer
			d.Control = func(_, address string, _ syscall.RawConn) error {
				return checkDial(host, literal, address)
			}
			if !literal {
				_, port, _ := net.SplitHostPort(addr)
				addresses, err := fetchLookupIP(ctx, host)
				if err != nil {
					return nil, err
				}
				for _, address := range addresses {
					target := net.JoinHostPort(address.IP.String(), port)
					if err := checkDial(host, literal, target); err != nil {
						return nil, err
					}
				}
				var last error
				for _, address := range addresses {
					conn, err := d.DialContext(ctx, network, net.JoinHostPort(address.IP.String(), port))
					if err == nil {
						return conn, nil
					}
					last = err
				}
				if last != nil {
					return nil, last
				}
				return nil, &FetchDenied{Reason: "network: host resolved to no addresses"}
			}
			return d.DialContext(ctx, network, addr)
		},
		ForceAttemptHTTP2:   true,
		TLSHandshakeTimeout: 10 * time.Second,
	}
	var rt http.RoundTripper = transport
	if fetchTestTransport != nil {
		rt = fetchTestTransport
	}
	return &http.Client{
		Transport: rt,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > MaxFetchRedirects {
				return fmt.Errorf("stopped after %d redirects", MaxFetchRedirects)
			}
			previous := via[len(via)-1].URL
			if previous.Scheme == "https" && req.URL.Scheme == "http" {
				return &FetchDenied{Reason: "network: HTTPS redirect downgrade refused"}
			}
			if redirectToOptOut(via[0].URL, req.URL) {
				required := FetchGrantPrefix + "<invalid-url>"
				if t, err := urlTarget(req.URL); err == nil {
					required = FetchGrantPrefix + t.String()
				}
				err := &FetchDenied{Reason: "network: redirect to an IP-literal or localhost destination refused", Required: required}
				return err
			}
			changedOrigin := !sameFetchOrigin(previous, req.URL)
			for i := 1; i < len(via); i++ {
				changedOrigin = changedOrigin || !sameFetchOrigin(via[i-1].URL, via[i].URL)
			}
			if changedOrigin {
				req.Header.Del("Authorization")
				req.Header.Del("Cookie")
			}
			grant, err := g.AuthorizeFetch(req.URL)
			if onHop != nil {
				onHop(req.URL, grant, err)
			}
			return err
		},
	}
}

// AsFetchDenied extracts a fetch denial from a client error, if any.
func AsFetchDenied(err error) (*FetchDenied, bool) {
	var d *FetchDenied
	if errors.As(err, &d) {
		return d, true
	}
	return nil, false
}

var fetchLookupIP = net.DefaultResolver.LookupIPAddr

var fetchTestTransport http.RoundTripper

// SetFetchTransportForTest replaces the network transport of every fetch
// client (authorization and redirect checks still run). Tests only; it
// returns a function that restores the previous transport.
func SetFetchTransportForTest(rt http.RoundTripper) func() {
	prev := fetchTestTransport
	fetchTestTransport = rt
	return func() { fetchTestTransport = prev }
}

// FetchGrantForURL returns the exact grant that authorizes rawURL, for
// static profiles (required-capabilities) and diagnostics.
func FetchGrantForURL(rawURL string) (string, bool) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", false
	}
	t, err := urlTarget(u)
	if err != nil {
		return "", false
	}
	return FetchGrantPrefix + t.String(), true
}

// DynamicFetchRequirement is the required-capabilities entry for a fetch
// whose URL is not a compile-time constant. It is not a valid grant: the
// operator must replace it with network:fetch=host[:port] entries.
const DynamicFetchRequirement = FetchGrantPrefix + "<dynamic>"

// sameFetchOrigin includes scheme and effective port, not just hostname.
func sameFetchOrigin(a, b *url.URL) bool {
	ta, ea := urlTarget(a)
	tb, eb := urlTarget(b)
	return ea == nil && eb == nil && strings.EqualFold(a.Scheme, b.Scheme) && ta.host == tb.host && ta.port == tb.port
}

// FetchPrivateOptOut describes explicit authority to reach private destinations.
func FetchPrivateOptOut(grant string) string {
	if !strings.HasPrefix(grant, FetchGrantPrefix) {
		return ""
	}
	t, err := parseFetchSpec(strings.TrimPrefix(grant, FetchGrantPrefix))
	if err == nil && (t.ip || t.host == "localhost") {
		return "explicit IP-literal or localhost grant"
	}
	return ""
}
