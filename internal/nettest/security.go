// Package nettest contains consumer-driven network conformance assertions shared
// by the capability client and the emitted Go helper.
package nettest

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type Lookup func(context.Context, string) ([]net.IPAddr, error)

func Run(t *testing.T, client func(...string) *http.Client, lookup func(Lookup) func()) {
	grant := func(raw string) string { u, _ := url.Parse(raw); return "network:fetch=" + u.Host }
	t.Run("RedirectToOptOut", func(t *testing.T) {
		c := client("network:fetch=api.example.com", "network:fetch=10.0.0.5", "network:fetch=localhost", "network:fetch=127.0.0.1", "network:fetch=[fd00::5]")
		hop := func(from, to string) error {
			first, _ := http.NewRequest("GET", from, nil)
			next, _ := http.NewRequest("GET", to, nil)
			return c.CheckRedirect(next, []*http.Request{first})
		}
		for _, tc := range [][2]string{
			{"http://api.example.com/", "http://10.0.0.5/"},
			{"http://api.example.com/", "http://localhost/"},
			{"http://api.example.com/", "http://[fd00::5]/"},
			{"http://localhost/", "http://127.0.0.1/"},
			{"http://127.0.0.1/", "http://localhost/"},
			{"http://10.0.0.5/", "http://127.0.0.1/"},
		} {
			if err := hop(tc[0], tc[1]); err == nil {
				t.Errorf("redirect %s -> %s allowed", tc[0], tc[1])
			}
		}
		for _, tc := range [][2]string{
			{"http://api.example.com/", "http://api.example.com/next"},
			{"http://10.0.0.5/", "http://10.0.0.5/next"},
			{"http://localhost/", "http://localhost/next"},
		} {
			if err := hop(tc[0], tc[1]); err != nil {
				t.Errorf("redirect %s -> %s refused: %v", tc[0], tc[1], err)
			}
		}
	})
	t.Run("RedirectCap11", func(t *testing.T) {
		var hits atomic.Int64
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hits.Add(1)
			var n int
			fmt.Sscanf(r.URL.Path, "/%d", &n)
			if n == 11 {
				io.WriteString(w, "escaped")
				return
			}
			http.Redirect(w, r, fmt.Sprintf("/%d", n+1), 302)
		}))
		defer srv.Close()
		c := client(grant(srv.URL))
		resp, err := c.Get(srv.URL + "/0")
		if resp != nil {
			resp.Body.Close()
		}
		if err == nil || hits.Load() != 11 {
			t.Fatalf("11-hop chain escaped cap: err=%v hits=%d", err, hits.Load())
		}
	})
	t.Run("ProxyEnvIgnored", func(t *testing.T) {
		var traps atomic.Int64
		trap := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { traps.Add(1) }))
		trap.Config.ConnState = func(_ net.Conn, s http.ConnState) {
			if s == http.StateNew {
				traps.Add(1)
			}
		}
		defer trap.Close()
		for _, key := range []string{"HTTPS_PROXY", "HTTP_PROXY", "https_proxy", "http_proxy", "ALL_PROXY"} {
			t.Setenv(key, trap.URL)
		}
		t.Setenv("NO_PROXY", "")
		t.Setenv("no_proxy", "")
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "direct") }))
		defer srv.Close()
		restore := lookup(func(context.Context, string) ([]net.IPAddr, error) {
			return []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, nil
		})
		defer restore()
		// localhost is the named opt-out; net/http ProxyFromEnvironment bypasses it.
		// An explicit non-loopback literal defeats that bypass without real DNS.
		c := client("network:fetch=192.0.2.1:80")
		tr := c.Transport.(*http.Transport)
		original := tr.DialContext
		tr.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			if strings.HasPrefix(addr, "192.0.2.1:") {
				addr = strings.TrimPrefix(srv.URL, "http://")
			}
			return original(ctx, network, addr)
		}
		resp, err := c.Get("http://192.0.2.1/")
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if string(b) != "direct" || traps.Load() != 0 {
			t.Fatalf("proxy used: body=%q trap connections=%d", b, traps.Load())
		}
		// Also exercise HTTPS_PROXY with a TLS destination.
		tlsSrv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "tls-direct") }))
		defer tlsSrv.Close()
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} // local test certificate only
		tr.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			if strings.HasPrefix(addr, "192.0.2.1:") {
				addr = strings.TrimPrefix(tlsSrv.URL, "https://")
			}
			return original(ctx, network, addr)
		}
		resp, err = c.Get("https://192.0.2.1/")
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if traps.Load() != 0 {
			t.Fatalf("HTTPS proxy received %d connections", traps.Load())
		}
	})
	t.Run("HTTPSDowngrade", func(t *testing.T) {
		var hits atomic.Int64
		dst := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1) }))
		defer dst.Close()
		src := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, dst.URL, 302) }))
		defer src.Close()
		c := client(grant(src.URL), grant(dst.URL))
		c.Transport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		resp, err := c.Get(src.URL)
		if resp != nil {
			resp.Body.Close()
		}
		if err == nil || hits.Load() != 0 {
			t.Fatalf("downgrade connected: err=%v hits=%d", err, hits.Load())
		}
	})
	t.Run("CredentialsPortChange", func(t *testing.T) {
		var credentials string
		var hits atomic.Int64
		dst := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			credentials += r.Header.Get("Authorization") + r.Header.Get("Cookie")
			hits.Add(1)
			if r.URL.Path != "/final" {
				http.Redirect(w, r, "/final", 307)
			}
		}))
		defer dst.Close()
		src := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, dst.URL, 307) }))
		defer src.Close()
		c := client(grant(src.URL), grant(dst.URL))
		req, _ := http.NewRequest("GET", src.URL, nil)
		req.Header.Set("Authorization", "Bearer secret")
		req.Header.Set("Cookie", "session=secret")
		resp, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if hits.Load() != 2 || credentials != "" {
			t.Fatalf("credentials crossed port: hits=%d credentials=%q", hits.Load(), credentials)
		}
	})

	t.Run("CredentialsSameOrigin", func(t *testing.T) {
		c := client("network:fetch=example.test")
		first, _ := http.NewRequest("GET", "http://EXAMPLE.test/", nil)
		next, _ := http.NewRequest("GET", "http://example.test:80/next", nil)
		next.Header.Set("Authorization", "Bearer secret")
		next.Header.Set("Cookie", "session=secret")
		if err := c.CheckRedirect(next, []*http.Request{first}); err != nil {
			t.Fatal(err)
		}
		if next.Header.Get("Authorization") != "Bearer secret" || next.Header.Get("Cookie") != "session=secret" {
			t.Fatal("same effective origin lost credentials")
		}
	})
	t.Run("NamePrivateDial", func(t *testing.T) {
		var hits atomic.Int64
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1) }))
		defer srv.Close()
		restore := lookup(func(context.Context, string) ([]net.IPAddr, error) {
			return []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, nil
		})
		defer restore()
		port := strings.TrimPrefix(srv.URL, "http://127.0.0.1:")
		c := client("network:fetch=private.test:" + port)
		resp, err := c.Get("http://private.test:" + port)
		if resp != nil {
			resp.Body.Close()
		}
		if err == nil || hits.Load() != 0 {
			t.Fatalf("name connected to loopback: err=%v hits=%d", err, hits.Load())
		}
		c = client("network:fetch=localhost:" + port)
		resp, err = c.Get("http://localhost:" + port)
		if err != nil {
			t.Fatalf("localhost opt-out: %v", err)
		}
		resp.Body.Close()
		c = client(grant(srv.URL))
		resp, err = c.Get(srv.URL)
		if err != nil {
			t.Fatalf("literal opt-out: %v", err)
		}
		resp.Body.Close()
		if hits.Load() != 2 {
			t.Fatalf("opt-outs failed: %d", hits.Load())
		}
	})

	t.Run("PrivateRangesDial", func(t *testing.T) {
		for _, ip := range BlockedAddresses {
			t.Run(ip, func(t *testing.T) {
				restore := lookup(func(context.Context, string) ([]net.IPAddr, error) { return []net.IPAddr{{IP: net.ParseIP(ip)}}, nil })
				defer restore()
				c := client("network:fetch=private.test:80")
				ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
				defer cancel()
				conn, err := c.Transport.(*http.Transport).DialContext(ctx, "tcp", "private.test:80")
				if conn != nil {
					conn.Close()
				}
				if err == nil || !(strings.Contains(err.Error(), "network:") || strings.Contains(err.Error(), "capability denied")) {
					t.Fatalf("missing private policy denial for %s: %v", ip, err)
				}
			})
		}
	})
	t.Run("MulticastDial", func(t *testing.T) {
		for _, address := range []string{"224.0.0.1:80", "[ff02::1]:80"} {
			c := client("network:fetch=" + address)
			_, err := c.Transport.(*http.Transport).DialContext(context.Background(), "tcp", address)
			if err == nil || !(strings.Contains(err.Error(), "network:") || strings.Contains(err.Error(), "capability denied")) {
				t.Fatalf("multicast lacks policy denial: %v", err)
			}
		}
	})
}

// BlockedAddresses must be refused for a hostname grant on every target.
var BlockedAddresses = []string{
	"127.1.2.3", "::1", "10.1.2.3", "172.16.0.1", "172.31.255.255", "192.168.1.1",
	"fc00::1", "fd12::1", "64:ff9b::7f00:1", "64:ff9b::808:808", "64:ff9b:1::1", "64:ff9b:1::808:808",
	"169.254.169.254", "fe80::1", "::ffff:127.0.0.1", "224.0.0.1", "ff00::1",
	"100.64.0.1", "100.100.100.200", "100.127.255.255", "::ffff:100.100.100.200",
	"0.1.2.3", "198.18.0.1", "198.19.255.255", "192.0.0.1", "240.0.0.1", "255.255.255.255",
	"fec0::1", "::7f00:1", "::a00:5", "2002:7f00:1::1", "2002:a00:5::1", "2002:6464:64c8::1",
	"2001:0:4136:e378::1", "100::1", "2001:db8::1", "::",
}

// PublicAddresses must stay reachable for a hostname grant on every target.
var PublicAddresses = []string{
	"8.8.8.8", "172.15.0.1", "172.32.0.1", "192.169.1.1", "100.63.255.255", "100.128.0.1",
	"198.17.255.255", "198.20.0.1", "2001:4860::1", "2606:4700::1", "2002:808:808::1", "::808:808",
}
