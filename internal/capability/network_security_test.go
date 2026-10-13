package capability

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
	"testing"

	"github.com/howlcipher/howlframe/internal/nettest"
)

func TestFetchSecurity(t *testing.T) {
	nettest.Run(t, func(raw ...string) *http.Client {
		var g Grants
		for _, s := range raw {
			g = append(g, Capability(s))
		}
		return FetchClient(g, nil)
	}, func(f nettest.Lookup) func() {
		old := fetchLookupIP
		fetchLookupIP = f
		return func() { fetchLookupIP = old }
	})
}
func TestFetchPrivateRanges(t *testing.T) {
	for _, ip := range []string{"127.1.2.3", "::1", "10.1.2.3", "172.16.0.1", "172.31.255.255", "192.168.1.1", "fc00::1", "fd12::1", "64:ff9b::7f00:1", "169.254.169.254", "fe80::1", "::ffff:127.0.0.1", "224.0.0.1", "ff00::1"} {
		if err := checkDial("private.test", false, net.JoinHostPort(ip, "80")); err == nil {
			t.Errorf("allowed %s", ip)
		}
	}
	for _, ip := range []string{"172.15.0.1", "172.32.0.1", "192.169.1.1", "2001:4860::1", "64:ff9b:1::1", "64:ff9b::1:0:0"} {
		if forbiddenResolved(netip.MustParseAddr(ip)) {
			t.Errorf("blocked public %s", ip)
		}
	}
}
func TestFetchTrailingDotDiagnostic(t *testing.T) {
	_, err := ParseGrant("network:fetch=example.com.")
	if err == nil || !strings.Contains(err.Error(), "trailing dot") || strings.Contains(err.Error(), "numeric") {
		t.Fatalf("inaccurate diagnostic: %v", err)
	}
}
func TestEnvGrantRefusesCodeLoading(t *testing.T) {
	for _, name := range []string{"PYTHONPATH", "NODE_OPTIONS", "BASH_ENV", "ENV", "PERL5LIB", "PERL5OPT", "RUBYOPT", "RUBYLIB", "PYTHONSTARTUP", "NODE_PATH"} {
		for _, raw := range []string{name, strings.ToLower(name)} {
			if _, err := ParseGrant(EnvGrantPrefix + raw); err == nil {
				t.Errorf("allowed %s", raw)
			}
			if len((Grants{Capability(EnvGrantPrefix + raw)}).ExecEnv(func(string) (string, bool) { return "injected", true })) != 0 {
				t.Errorf("raw grant allowed %s", raw)
			}
		}
	}
}
