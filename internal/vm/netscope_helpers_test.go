package vm

import (
	"net/url"
	"testing"

	"github.com/howlcipher/howlframe/internal/capability"
)

// fetchGrantFor returns the host-scoped grant that authorizes rawURL (#165).
func fetchGrantFor(t *testing.T, rawURL string) capability.Capability {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	g, err := capability.ParseGrant(capability.FetchGrantPrefix + u.Host)
	if err != nil {
		t.Fatal(err)
	}
	return g
}
