package vm

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/howlcipher/howlframe/internal/capability"
)

// Regression for audit finding HF-04: with (serialize_requests), one client that
// stops reading a large response must not stall unrelated requests, and the
// stalled connection must be released once the write timeout passes.

func freeLoopbackPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// startTestServer runs src (an http_server program) on port in the background.
// The server has no shutdown; it ends with the test process, bound to loopback.
func startTestServer(t *testing.T, port int, src string) {
	t.Helper()
	_, prog := parseAndCompile(t, src)
	policy := DefaultExecutionPolicy()
	policy.Limits.MaxInstructions = 50_000_000
	policy.Limits.MaxStartupInstructions = 50_000_000
	policy.Limits.HTTPListenHost = "127.0.0.1"
	go func() {
		var out, errOut bytes.Buffer
		RunBytecodeWithEvidence(prog, nil, policy, []capability.Capability{capability.Network}, strings.NewReader(""), &out, &errOut, 0)
	}()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 200*time.Millisecond)
		if err == nil {
			c.Close()
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("test server did not start")
}

const bigBodyRoute = `(route "/big" (lambda (req)
    (let (s "xxxxxxxx")
      (do (for i (list 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18) (set s (str s s)))
          (res 200 "text/plain" s)))))`

func healthWithin(port int, limit time.Duration) (int, time.Duration, error) {
	client := &http.Client{Timeout: limit}
	start := time.Now()
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/health", port))
	if err != nil {
		return 0, time.Since(start), err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	return resp.StatusCode, time.Since(start), nil
}

func stalledClient(t *testing.T, port, pipelined int) net.Conn {
	t.Helper()
	conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		t.Fatal(err)
	}
	if tcp, ok := conn.(*net.TCPConn); ok {
		tcp.SetReadBuffer(4096)
	}
	req := "GET /big HTTP/1.1\r\nHost: x\r\n\r\n"
	if _, err := conn.Write([]byte(strings.Repeat(req, pipelined))); err != nil {
		t.Fatal(err)
	}
	return conn
}

func TestSerializedServerSurvivesNonReadingClient(t *testing.T) {
	saved := responseWriteTimeout
	responseWriteTimeout = 2 * time.Second
	t.Cleanup(func() { responseWriteTimeout = saved })

	port := freeLoopbackPort(t)
	startTestServer(t, port, fmt.Sprintf(`(http_server %d (serialize_requests)
  %s
  (route "/health" (lambda (req) (res 200 "text/plain" "ok"))))`, port, bigBodyRoute))

	if code, _, err := healthWithin(port, 3*time.Second); err != nil || code != 200 {
		t.Fatalf("baseline health failed: %d %v", code, err)
	}

	stalled := stalledClient(t, port, 12)
	defer stalled.Close()
	time.Sleep(500 * time.Millisecond) // let the server start writing to the stalled client

	code, took, err := healthWithin(port, 1500*time.Millisecond)
	if err != nil || code != 200 {
		t.Fatalf("an unrelated request was blocked by a client that stopped reading: status=%d after %v: %v", code, took, err)
	}

	// Stay stalled past the write timeout. The server must then give up on this
	// client and close the connection, having sent only part of what was asked:
	// draining afterwards must reach end-of-file, not hang, and not deliver all
	// twelve 2 MiB responses.
	time.Sleep(responseWriteTimeout + 1500*time.Millisecond)
	if tcp, ok := stalled.(*net.TCPConn); ok {
		tcp.SetReadBuffer(1 << 20) // widen the tiny window so draining is not artificially slow
	}
	stalled.SetReadDeadline(time.Now().Add(15 * time.Second))
	received, err := io.Copy(io.Discard, stalled)
	if ne, ok := err.(net.Error); ok && ne.Timeout() {
		t.Fatalf("stalled connection was not released after the write timeout (received %d bytes): %v", received, err)
	}
	if full := int64(12 * 8 * (1 << 18)); received >= full {
		t.Fatalf("server delivered every response (%d bytes) to a client that never read them in time", received)
	}

	// Normal responses are still complete and correct afterwards.
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/big", port))
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil || resp.StatusCode != 200 || len(body) != 8*(1<<18) {
		t.Fatalf("large valid response damaged: status=%d len=%d err=%v", resp.StatusCode, len(body), err)
	}
}
