package main
import "testing"
func add(a int64, b int64) int64 { return a + b }
func TestAdd(t *testing.T) {
    if add(2, 3) != 5 { t.Errorf("expected 5") }
}
