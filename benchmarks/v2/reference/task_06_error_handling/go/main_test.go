package main

import (
	"errors"
	"testing"
)

func divide(a, b int) (int, error) {
	if b == 0 {
		return 0, errors.New("Division by zero")
	}
	return a / b, nil
}
func TestDivide(t *testing.T) {
	_, err := divide(10, 0)
	if err == nil || err.Error() != "Division by zero" {
		t.Errorf("expected Division by zero error")
	}
}
