package main

import (
	"strings"
	"sync"
)

// syncBuffer is an io.Writer whose contents can be read while a subprocess is
// still writing to it. os/exec copies a child's output on its own goroutine, so
// a plain bytes.Buffer read from the test goroutine is a data race (issue #192).
type syncBuffer struct {
	mu sync.Mutex
	b  strings.Builder
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}
