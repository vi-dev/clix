package clix

import (
	"os"
	"sync/atomic"

	"golang.org/x/term"
)

var getSize = term.GetSize

type widthCache struct{ w atomic.Int32 }

func (c *widthCache) get() int {
	if w := c.w.Load(); w > 0 {
		return int(w)
	}
	w, _, err := getSize(int(os.Stderr.Fd()))
	if err != nil || w <= 0 {
		w = 80
	}
	c.w.Store(int32(w))
	return w
}

func (c *widthCache) reset() { c.w.Store(0) }
