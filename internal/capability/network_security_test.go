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
	for _, ip := range append(nettest.BlockedAddresses, "224.0.0.1", "ff00::1") {
		if err := checkDial("private.test", false, net.JoinHostPort(ip, "80")); err == nil {
			t.Errorf("allowed %s", ip)
		}
	}
	for _, ip := range nettest.PublicAddresses {
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
	for _, raw := range nettest.EnvRefused {
		if _, err := ParseGrant(EnvGrantPrefix + raw); err == nil {
			t.Errorf("allowed %s", raw)
		}
		if len((Grants{Capability(EnvGrantPrefix + raw)}).ExecEnv(func(string) (string, bool) { return "injected", true })) != 0 {
			t.Errorf("raw grant allowed %s", raw)
		}
	}
	for _, raw := range nettest.EnvAllowed {
		if _, err := ParseGrant(EnvGrantPrefix + raw); err != nil {
			t.Errorf("refused benign %s: %v", raw, err)
		}
		if env := (Grants{Capability(EnvGrantPrefix + raw)}).ExecEnv(func(string) (string, bool) { return "ok", true }); len(env) != 1 {
			t.Errorf("benign %s not passed: %v", raw, env)
		}
	}
}
