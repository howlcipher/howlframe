package harness

// MaterializeReferenceSandbox exposes the existing evidence-only materializer
// for offline, trusted reference-program measurements. It provides no OS isolation.
// The run directory must be private and empty; oracles are never copied.
func MaterializeReferenceSandbox(root, run string, t Task) (sandbox, canary string, err error) {
	if err := (Runner{}).validateTask(t); err != nil {
		return "", "", err
	}
	return materialize(root, run, t, "http://127.0.0.1:1")
}
