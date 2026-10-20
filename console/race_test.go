package console

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func (b *syncBuffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Len()
}

type terminalWrite struct {
	stream string
	text   string
}

type terminal struct {
	mu     sync.Mutex
	writes []terminalWrite
}

type terminalStream struct {
	t    *terminal
	name string
}

func (s terminalStream) Write(p []byte) (int, error) {
	s.t.mu.Lock()
	defer s.t.mu.Unlock()
	s.t.writes = append(s.t.writes, terminalWrite{stream: s.name, text: string(p)})
	return len(p), nil
}

func (t *terminal) snapshot() []terminalWrite {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]terminalWrite(nil), t.writes...)
}

func streamText(writes []terminalWrite, stream string) string {
	var b strings.Builder
	for _, w := range writes {
		if w.stream == stream {
			b.WriteString(w.text)
		}
	}
	return b.String()
}

var cursorUp = regexp.MustCompile(`^\x1b\[(\d+)A`)

func rowInsideBlock(writes []terminalWrite) string {
	shown, below := 0, 0
	for i, w := range writes {
		if w.stream == "stdout" {
			if shown+below > 0 {
				return fmt.Sprintf("write %d: stdout %q landed with %d block lines on screen", i, w.text, shown+below)
			}
			continue
		}
		s := w.text
		for s != "" {
			switch {
			case cursorUp.MatchString(s):
				m := cursorUp.FindStringSubmatch(s)
				n, _ := strconv.Atoi(m[1])
				shown = max(shown-n, 0)
				below += n
				s = s[len(m[0]):]
			case strings.HasPrefix(s, "\x1b[J"):
				below = 0
				s = s[len("\x1b[J"):]
			default:
				end := strings.IndexByte(s, '\n')
				if end < 0 {
					end = len(s) - 1
				}
				if strings.HasPrefix(s, "\x1b[2K") {
					shown++
					below = max(below-1, 0)
				} else if shown+below > 0 {
					return fmt.Sprintf("write %d: stderr line %q landed inside the block", i, s[:end+1])
				}
				s = s[end+1:]
			}
		}
	}
	return ""
}

var blockRunStart = regexp.MustCompile(`(?:\x1b\[\d+A|\x1b\[J|\x1b\[2K[^\n]*\n)$`)

func TestProgressNeverContendsWithRepaints(t *testing.T) {
	var out, errb syncBuffer
	c := New(&out, &errb, Options{IsTTY: true, Color: ColorNever})
	tk := c.Start("Copying")
	tk.Phase("copying")
	inner := tk.(*task)

	c.mu.Lock()
	inner.mu.Lock()
	unblocked := make(chan struct{})
	go func() {
		for i := range 1000 {
			tk.Progress(int64(i), 1000, Items)
		}
		close(unblocked)
	}()
	select {
	case <-unblocked:
	case <-time.After(time.Second):
	}
	inner.mu.Unlock()
	c.mu.Unlock()
	select {
	case <-unblocked:
	default:
		t.Fatal("Progress blocked while the console and task mutexes were held")
	}

	const total = 100000
	start := time.Now()
	var stop atomic.Bool
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			for !stop.Load() {
				c.Tick()
			}
		})
	}
	writer := make(chan struct{})
	go func() {
		defer close(writer)
		for i := int64(1); i <= total; i++ {
			tk.Progress(i, total, Items)
		}
	}()
	select {
	case <-writer:
	case <-time.After(2 * time.Second):
		stop.Store(true)
		wg.Wait()
		t.Fatal("100000 Progress calls did not finish within 2s while repaints ran")
	}
	stop.Store(true)
	wg.Wait()
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("Progress with concurrent repaints took %v, want under 2s", elapsed)
	}

	c.Tick()
	got := errb.String()
	i := strings.LastIndex(got, "\x1b[2K")
	if i < 0 {
		t.Fatalf("no repaint in stderr: %q", got)
	}
	last := strings.TrimSuffix(got[i+len("\x1b[2K"):], "\n")
	if want := fmt.Sprintf("copying %d/%d", total, total); !strings.HasSuffix(last, want) {
		t.Errorf("last repainted line = %q, want it to end with %q", last, want)
	}
	tk.Done("Copied")
}

func TestPhaseResetsProgressAtomically(t *testing.T) {
	var out, errb syncBuffer
	c := New(&out, &errb, Options{IsTTY: true, Color: ColorNever})
	tk := c.Start("Filling")
	inner := tk.(*task)
	allowed := map[string]bool{"a": true, "a 0/1000": true, "a 900/1000": true, "b": true, "b 0/2": true, "b 1/2": true}

	var stop atomic.Bool
	var bad atomic.Value
	var wg sync.WaitGroup
	for range 3 {
		wg.Go(func() {
			for !stop.Load() {
				phase, trailing := inner.snapshot(time.Now())
				if phase != "" && !allowed[trailing] {
					bad.Store(trailing)
				}
			}
		})
	}
	wg.Go(func() {
		for !stop.Load() {
			c.Tick()
		}
	})
	for range 2000 {
		tk.Phase("a")
		tk.Progress(900, 1000, Items)
		tk.Phase("b")
		tk.Progress(1, 2, Items)
	}
	stop.Store(true)
	wg.Wait()
	tk.Done("Filled")

	if v := bad.Load(); v != nil {
		t.Errorf("snapshot showed %q, want only a phase with its own progress", v)
	}
	counts := regexp.MustCompile(`  ([ab]) (\d+)/(\d+)\n`)
	rendered := counts.FindAllStringSubmatch(stripANSI(errb.String()), -1)
	if len(rendered) == 0 {
		t.Fatal("no repaint showed a phase with progress")
	}
	for _, m := range rendered {
		if trailing := m[1] + " " + m[2] + "/" + m[3]; !allowed[trailing] {
			t.Errorf("rendered %q, want only a phase with its own progress", trailing)
		}
	}
}

func TestDataRowsNeverLandInsideTheBlock(t *testing.T) {
	var term terminal
	c := New(terminalStream{t: &term, name: "stdout"}, terminalStream{t: &term, name: "stderr"},
		Options{IsTTY: true, OutIsTTY: true, Color: ColorNever})
	first, second := c.Start("Listing"), c.Start("Indexing")
	first.Phase("reading")
	second.Phase("scanning")

	var stop atomic.Bool
	var ticks sync.WaitGroup
	ticks.Go(func() {
		for !stop.Load() {
			c.Tick()
		}
	})
	var rows sync.WaitGroup
	for g := range 4 {
		rows.Go(func() {
			for i := range 50 {
				c.Data("row %d-%d\n", g, i)
				first.Progress(int64(i), 50, Items)
			}
		})
	}
	rows.Wait()
	stop.Store(true)
	ticks.Wait()
	writes := term.snapshot()
	first.Done("Listed")
	second.Done("Indexed")

	if msg := rowInsideBlock(writes); msg != "" {
		t.Error(msg)
	}
	stderr := streamText(writes, "stderr")
	for i := 0; ; {
		j := strings.Index(stderr[i:], "\x1b[2K")
		if j < 0 {
			break
		}
		at := i + j
		if at > 0 && !blockRunStart.MatchString(stderr[max(at-256, 0):at]) {
			t.Fatalf("block line at byte %d does not follow a cursor-up, a clear or another block line: %q", at, stderr[max(at-40, 0):at])
		}
		i = at + len("\x1b[2K")
	}
	stdout := streamText(writes, "stdout")
	lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
	if len(lines) != 200 {
		t.Errorf("stdout has %d lines, want 200", len(lines))
	}
	for g := range 4 {
		for i := range 50 {
			if n := strings.Count(stdout, fmt.Sprintf("row %d-%d\n", g, i)); n != 1 {
				t.Errorf("row %d-%d appears %d times, want 1", g, i, n)
			}
		}
	}
}

func TestExitDuringNarrationNeitherHangsNorPanics(t *testing.T) {
	var out, errb syncBuffer
	c := New(&out, &errb, Options{IsTTY: true, Color: ColorNever, Verbose: true})
	open := c.Start("Building")
	open.Phase("compiling")

	var exited atomic.Bool
	var afterExit atomic.Int64
	stop := make(chan struct{})
	var wg sync.WaitGroup
	for g := range 8 {
		wg.Go(func() {
			for i := 0; ; i++ {
				select {
				case <-stop:
					return
				default:
				}
				if exited.Load() {
					afterExit.Add(1)
				}
				c.Infof("step %d-%d", g, i)
				c.Warn("careful")
				c.Debug("detail")
				c.BlankErr()
				tk := c.Start(fmt.Sprintf("job %d-%d", g, i))
				tk.Phase("running")
				tk.Infof("inside %d", i)
				open.Progress(int64(i), 1000, Items)
				c.Tick()
				if i%2 == 0 {
					tk.Done("job done")
				} else {
					tk.Fail("job failed")
				}
			}
		})
	}
	joined := make(chan struct{})
	finish := func() {
		close(stop)
		go func() {
			wg.Wait()
			close(joined)
		}()
		select {
		case <-joined:
		case <-time.After(2 * time.Second):
			t.Fatal("narrating goroutines did not stop within 2s")
		}
	}

	deadline := time.Now().Add(time.Second)
	for errb.Len() < 4096 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	returned := make(chan int)
	go func() {
		c.Exit(130, nil)
		exited.Store(true)
		returned <- errb.Len()
	}()
	var atExit int
	select {
	case atExit = <-returned:
	case <-time.After(2 * time.Second):
		finish()
		t.Fatal("Exit did not return within 2s during narration")
	}

	deadline = time.Now().Add(2 * time.Second)
	for afterExit.Load() < 800 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	finish()
	open.Done("Built")
	if n := afterExit.Load(); n < 800 {
		t.Errorf("only %d narration rounds ran after Exit, want at least 800", n)
	}
	if got := errb.String(); len(got) != atExit {
		t.Errorf("stderr grew after Exit returned: %q", got[atExit:])
	}
}

func TestTickerStopIsByGeneration(t *testing.T) {
	var out, errb syncBuffer
	c := New(&out, &errb, Options{IsTTY: true, Color: ColorNever})
	base := settledGoroutines()
	for i := range 50 {
		tk := c.Start(fmt.Sprintf("task %d", i))
		tk.Phase("working")
		tk.Done(fmt.Sprintf("done %d", i))
	}
	if n := waitGoroutines(base); n != base {
		t.Errorf("goroutines = %d after the last task, want %d", n, base)
	}
	c.mu.Lock()
	ticks := c.ticks
	c.mu.Unlock()
	before := errb.String()

	time.Sleep(2 * tickInterval)

	c.mu.Lock()
	later := c.ticks
	c.mu.Unlock()
	if later != ticks {
		t.Errorf("%d ticks ran after the last task, want none", later-ticks)
	}
	got := errb.String()
	if got != before {
		t.Errorf("stderr grew after the last task: %q", strings.TrimPrefix(got, before))
	}
	if !strings.HasSuffix(got, "OK done 49\n") {
		t.Errorf("stderr ends with %q, want the last OK line", got[max(len(got)-40, 0):])
	}
	for i := range 50 {
		if n := strings.Count(got, fmt.Sprintf("OK done %d\n", i)); n != 1 {
			t.Errorf("OK done %d appears %d times, want 1", i, n)
		}
	}
}

func TestTaskTerminalCallsFromManyGoroutinesPrintOnce(t *testing.T) {
	for _, opts := range []Options{
		{IsTTY: true, Color: ColorNever},
		{Color: ColorNever},
	} {
		var out, errb syncBuffer
		c := New(&out, &errb, opts)
		for round := range 20 {
			tk := c.Start(fmt.Sprintf("task %d", round))
			tk.Phase("working")
			gate := make(chan struct{})
			var wg sync.WaitGroup
			for g := range 16 {
				wg.Go(func() {
					<-gate
					if g%2 == 0 {
						tk.Done(fmt.Sprintf("done %d", round))
					} else {
						tk.Fail(fmt.Sprintf("failed %d", round))
					}
				})
			}
			close(gate)
			wg.Wait()
			got := stripANSI(errb.String())
			ok := strings.Count(got, fmt.Sprintf("OK done %d\n", round))
			failed := strings.Count(got, fmt.Sprintf("ERROR failed %d\n", round))
			if ok+failed != 1 {
				t.Errorf("IsTTY=%v round %d: %d OK and %d ERROR lines, want exactly one completion line", opts.IsTTY, round, ok, failed)
			}
		}
	}
}
