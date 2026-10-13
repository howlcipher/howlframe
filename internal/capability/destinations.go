package capability

import (
	"net/netip"
	"net/url"
)

// SpecialPrefixes is the single table of destinations a hostname grant may
// not reach (#172, #177). Generated Go and JavaScript render this same table,
// so the three targets cannot drift. IPv4 entries also match IPv4-mapped
// IPv6; IPv4 embedded in IPv4-compatible (::a.b.c.d) and 6to4 (2002::/16)
// addresses is decoded and checked against the IPv4 rows. NAT64 and Teredo
// are refused outright because the embedded address is attacker-chosen.
var SpecialPrefixes = []string{
	"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8", "169.254.0.0/16",
	"172.16.0.0/12", "192.0.0.0/24", "192.0.2.0/24", "192.88.99.0/24", "192.168.0.0/16",
	"198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "224.0.0.0/4", "240.0.0.0/4",
	"::/128", "::1/128", "64:ff9b::/96", "64:ff9b:1::/48", "100::/64", "2001::/32",
	"2001:db8::/32", "fc00::/7", "fe80::/10", "fec0::/10", "ff00::/8",
}

var specialPrefixes = func() []netip.Prefix {
	out := make([]netip.Prefix, len(SpecialPrefixes))
	for i, s := range SpecialPrefixes {
		out[i] = netip.MustParsePrefix(s)
	}
	return out
}()

// ForbiddenDestination reports whether a hostname grant may not connect to a.
func ForbiddenDestination(a netip.Addr) bool {
	a = a.Unmap()
	if a.Is6() {
		b := a.As16()
		if b[0] == 0x20 && b[1] == 0x02 { // 6to4
			if ForbiddenDestination(netip.AddrFrom4([4]byte{b[2], b[3], b[4], b[5]})) {
				return true
			}
		}
		compat := true
		for _, x := range b[:12] {
			compat = compat && x == 0
		}
		if compat && !(b[12] == 0 && b[13] == 0 && b[14] == 0) { // ::a.b.c.d
			return ForbiddenDestination(netip.AddrFrom4([4]byte{b[12], b[13], b[14], b[15]}))
		}
	}
	if !a.IsGlobalUnicast() {
		return true
	}
	for _, p := range specialPrefixes {
		if p.Contains(a) {
			return true
		}
	}
	return false
}

// optOutHost reports an explicit opt-out destination: an IP literal or localhost.
func optOutHost(u *url.URL) (string, bool) {
	t, err := urlTarget(u)
	if err != nil {
		return "", false
	}
	return t.host, t.ip || t.host == "localhost"
}

// redirectToOptOut refuses a redirect that lands on an opt-out destination
// other than the host the request started on: a hostname grant must not be
// able to bounce into a granted private IP or localhost, and localhost must
// not bounce into a granted 127.0.0.1.
func redirectToOptOut(first, next *url.URL) bool {
	host, opt := optOutHost(next)
	if !opt {
		return false
	}
	start, err := urlTarget(first)
	return err != nil || start.host != host
}
