package capability

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
)

func TestParseFetchGrant(t *testing.T) {
	for raw, want := range map[string]string{
		"network:fetch=example.com":         "network:fetch=example.com",
		"network:fetch=Example.COM:8080":    "network:fetch=example.com:8080",
		"network:fetch=127.0.0.1":           "network:fetch=127.0.0.1",
		"network:fetch=[::1]:80":            "network:fetch=[::1]:80",
		"network:fetch=[0:0::1]":            "network:fetch=[::1]",
		"network:fetch=169.254.169.254:80":  "network:fetch=169.254.169.254:80",
		"network:fetch=api.example.co.uk:1": "network:fetch=api.example.co.uk:1",
	} {
		got, err := ParseGrant(raw)
		if err != nil || string(got) != want {
			t.Errorf("ParseGrant(%q) = %q, %v; want %q", raw, got, err, want)
		}
	}
	for _, raw := range []string{
		"network:fetch=", "network:fetch=*.example.com", "network:fetch=*", "network:fetch=http://example.com",
		"network:fetch=example.com/path", "network:fetch=user@example.com", "network:fetch=example.com.",
		"network:fetch=.example.com", "network:fetch=127.1", "network:fetch=0x7f.0.0.1", "network:fetch=2130706433",
		"network:fetch=0177.0.0.1", "network:fetch=example.com:0", "network:fetch=example.com:",
		"network:fetch=example.com:65536", "network:fetch=example.com:08", "network:fetch=::1",
		"network:fetch=[fe80::1%eth0]", "network:fetch=exa mple.com", "network:fetch=example.com:80:80",
		"network:fetch=%31%32%37.0.0.1",
	} {
		if g, err := ParseGrant(raw); err == nil {
			t.Errorf("ParseGrant(%q) = %q, want error", raw, g)
		}
	}
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestAuthorizeFetchExactHost(t *testing.T) {
	g := Grants{"network:fetch=example.com", "network:fetch=api.test:8443", "network:fetch=10.0.0.5", "network:fetch=[::1]:9000"}
	allowed := []string{
		"http://example.com/", "https://example.com/x?y=z", "http://EXAMPLE.com:80/", "https://example.com:443/",
		"https://api.test:8443/v1", "http://10.0.0.5/", "http://[::1]:9000/", "http://[::ffff:10.0.0.5]/",
		"http://user:pw@example.com/",
	}
	for _, raw := range allowed {
		if grant, err := g.AuthorizeFetch(mustURL(t, raw)); err != nil || grant == "" {
			t.Errorf("AuthorizeFetch(%q) denied: %v", raw, err)
		}
	}
	denied := map[string]string{
		"http://evilexample.com/":                  "network:fetch=evilexample.com:80",
		"http://a.example.com/":                    "network:fetch=a.example.com:80",
		"http://example.com.evil.net/":             "network:fetch=example.com.evil.net:80",
		"http://example.com.":                      "network:fetch=<invalid-url>",
		"http://example.com:8080/":                 "network:fetch=example.com:8080",
		"http://api.test/":                         "network:fetch=api.test:80",
		"https://api.test/":                        "network:fetch=api.test:443",
		"http://10.0.0.6/":                         "network:fetch=10.0.0.6:80",
		"http://10.0.0.5:81/":                      "network:fetch=10.0.0.5:81",
		"http://169.254.169.254/latest/meta-data/": "network:fetch=169.254.169.254:80",
		"http://example.com@169.254.169.254/":      "network:fetch=169.254.169.254:80",
		"http://[fd00:ec2::254]/":                  "network:fetch=[fd00:ec2::254]:80",
		"http://0xa.0.0.5/":                        "network:fetch=<invalid-url>",
		"http://167772165/":                        "network:fetch=<invalid-url>",
		"http://010.0.0.5/":                        "network:fetch=<invalid-url>",
		"ftp://example.com/":                       "network:fetch=<invalid-url>",
		"file:///etc/passwd":                       "network:fetch=<invalid-url>",
		"//example.com/":                           "network:fetch=<invalid-url>",
		"http://[::1%25lo]:9000/":                  "network:fetch=<invalid-url>",
	}
	for raw, want := range denied {
		_, err := g.AuthorizeFetch(mustURL(t, raw))
		d, ok := AsFetchDenied(err)
		if !ok {
			t.Errorf("AuthorizeFetch(%q) allowed", raw)
			continue
		}
		if d.Required != want {
			t.Errorf("AuthorizeFetch(%q) required %q, want %q", raw, d.Required, want)
		}
		if strings.Contains(d.Required, "meta-data") || strings.Contains(d.Reason, "meta-data") {
			t.Errorf("denial for %q leaked the path", raw)
		}
	}
	// The coarse grant never fetches.
	if _, err := (Grants{Network}).AuthorizeFetch(mustURL(t, "http://example.com/")); err == nil {
		t.Error("coarse network authorized a fetch")
	}
	if (Grants{Network}).HasFetch() || !g.HasFetch() {
		t.Error("HasFetch mismatch")
	}
}

func TestFetchClientRechecksRedirects(t *testing.T) {
	var targetHits atomic.Int64
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetHits.Add(1)
		io.WriteString(w, "target")
	}))
	defer target.Close()
	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/hop", http.StatusFound)
	}))
	defer redirector.Close()
	rHost := strings.TrimPrefix(redirector.URL, "http://")
	tHost := strings.TrimPrefix(target.URL, "http://")

	var hops []string
	g := Grants{Capability(FetchGrantPrefix + rHost)}
	resp, err := FetchClient(g, func(_ *url.URL, grant string, err error) { hops = append(hops, grant) }).Get(redirector.URL)
	if resp != nil {
		resp.Body.Close()
	}
	if d, ok := AsFetchDenied(err); !ok || d.Required != FetchGrantPrefix+tHost || targetHits.Load() != 0 {
		t.Fatalf("ungranted hop: err=%v hits=%d", err, targetHits.Load())
	}

	g = append(g, Capability(FetchGrantPrefix+tHost))
	resp, err = FetchClient(g, nil).Get(redirector.URL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if targetHits.Load() != 1 {
		t.Fatalf("granted hop hits = %d", targetHits.Load())
	}
}

func TestForbiddenResolved(t *testing.T) {
	for _, a := range []string{"169.254.169.254", "fe80::1", "0.0.0.0", "::", "::ffff:169.254.169.254", "224.0.0.1"} {
		if !forbiddenResolved(netip.MustParseAddr(a)) {
			t.Errorf("%s not forbidden", a)
		}
	}
	for _, a := range []string{"93.184.216.34", "2001:4860:4860::8888"} {
		if forbiddenResolved(netip.MustParseAddr(a)) {
			t.Errorf("%s forbidden", a)
		}
	}
}

func TestEnvGrantRefusesDynamicLoader(t *testing.T) {
	for _, name := range []string{"LD_PRELOAD", "LD_LIBRARY_PATH", "LD_AUDIT", "ld_preload", "DYLD_INSERT_LIBRARIES", "Dyld_Library_Path"} {
		if _, err := ParseGrant(EnvGrantPrefix + name); err == nil {
			t.Errorf("process:env=%s accepted", name)
		}
		if env := (Grants{Capability(EnvGrantPrefix + name)}).ExecEnv(func(string) (string, bool) { return "x", true }); len(env) != 0 {
			t.Errorf("ExecEnv passed %s", name)
		}
	}
	// #177: HOME steers config/startup lookup and is refused; LANG still passes.
	if _, err := ParseGrant(EnvGrantPrefix + "HOME"); err == nil {
		t.Error("process:env=HOME accepted")
	}
	if _, err := ParseGrant(EnvGrantPrefix + "LANG"); err != nil {
		t.Error(err)
	}
	if _, err := ParseGrant(EnvGrantPrefix + "OLD_PATH"); err != nil {
		t.Error(err)
	}
}

func TestCheckDialRefusesRebindToLinkLocal(t *testing.T) {
	if _, ok := AsFetchDenied(checkDial("metadata.example", false, "169.254.169.254:80")); !ok {
		t.Error("DNS name resolving to 169.254.169.254 was dialed")
	}
	if _, ok := AsFetchDenied(checkDial("v6.example", false, "[fe80::1]:80")); !ok {
		t.Error("DNS name resolving to fe80::1 was dialed")
	}
	if err := checkDial("api.example", false, "93.184.216.34:443"); err != nil {
		t.Error(err)
	}
	// An explicitly granted IP literal is the operator's choice.
	if err := checkDial("169.254.169.254", true, "169.254.169.254:80"); err != nil {
		t.Error(err)
	}
}
