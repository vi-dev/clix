package consoletest

import (
	"bytes"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/vi-dev/clix/console"
)

type clock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *clock) read() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *clock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// Transcript runs fn on a console made with opts and returns what it printed to stderr. Each call to tick advances the clock by 100 milliseconds and moves narration on by one step.
func Transcript(t testing.TB, opts console.Options, fn func(c *console.Console, tick func())) string {
	t.Helper()
	clk := &clock{now: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)}
	opts.Now = clk.read
	opts.ManualTick = true
	var stderr bytes.Buffer
	c := console.New(io.Discard, &stderr, opts)
	fn(c, func() {
		clk.advance(100 * time.Millisecond)
		c.Tick()
	})
	return stderr.String()
}
