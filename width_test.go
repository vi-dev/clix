package clix

import (
	"errors"
	"testing"
)

func swapGetSize(t *testing.T, fn func(int) (int, int, error)) {
	t.Helper()
	orig := getSize
	getSize = fn
	t.Cleanup(func() { getSize = orig })
}

func TestWidthCacheFallsBackTo80OffTerminal(t *testing.T) {
	calls := 0
	swapGetSize(t, func(int) (int, int, error) {
		calls++
		return 0, 0, errors.New("not a terminal")
	})
	var c widthCache
	if got := c.get(); got != 80 {
		t.Fatalf("get() = %d, want 80", got)
	}
	if got := c.get(); got != 80 {
		t.Fatalf("second get() = %d, want 80", got)
	}
	if calls != 1 {
		t.Fatalf("getSize called %d times, want 1", calls)
	}
}

func TestWidthCacheResetForgets(t *testing.T) {
	width := 120
	calls := 0
	swapGetSize(t, func(int) (int, int, error) {
		calls++
		return width, 24, nil
	})
	var c widthCache
	if got := c.get(); got != 120 {
		t.Fatalf("get() = %d, want 120", got)
	}
	width = 100
	if got := c.get(); got != 120 {
		t.Fatalf("cached get() = %d, want 120", got)
	}
	c.reset()
	if got := c.get(); got != 100 {
		t.Fatalf("get() after reset = %d, want 100", got)
	}
	if calls != 2 {
		t.Fatalf("getSize called %d times, want 2", calls)
	}
}

func TestWidthCacheNonPositiveFallsBackTo80(t *testing.T) {
	swapGetSize(t, func(int) (int, int, error) { return 0, 0, nil })
	var c widthCache
	if got := c.get(); got != 80 {
		t.Fatalf("get() = %d, want 80", got)
	}
}
