package console

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// Align sets which side of its column a cell keeps to.
type Align int

const (
	// Left keeps cells to the left.
	Left Align = iota
	// Right keeps cells to the right.
	Right
)

// Column describes one column of a TableWriter.
type Column struct {
	// Name is the header, printed in upper case.
	Name string
	// Width is the column width in characters; zero means the header's width, or no limit for the last column.
	Width int
	// Align sets the side cells keep to; the zero value is Left.
	Align Align
}

// TableWriter prints a table to stdout one row at a time.
type TableWriter struct {
	c             *Console
	cols          []Column
	widths        []int
	unboundedLast bool
	mu            sync.Mutex
	started       bool
}

func (c *Console) writeData(p []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.r.beforeData()
	_, _ = c.out.Write(p)
}

// Data prints text formatted like fmt.Sprintf to stdout.
func (c *Console) Data(format string, a ...any) {
	c.writeData([]byte(fmt.Sprintf(format, a...)))
}

// JSON prints v to stdout as indented JSON.
func (c *Console) JSON(v any) error {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return err
	}
	c.writeData(b.Bytes())
	return nil
}

// JSONLine prints v to stdout as JSON on one line.
func (c *Console) JSONLine(v any) error {
	var b bytes.Buffer
	if err := json.NewEncoder(&b).Encode(v); err != nil {
		return err
	}
	c.writeData(b.Bytes())
	return nil
}

// Blank prints a blank line to stdout.
func (c *Console) Blank() {
	c.writeData([]byte("\n"))
}

// TableWriter returns a table with the given columns that prints to stdout one row at a time.
func (c *Console) TableWriter(cols ...Column) *TableWriter {
	w := &TableWriter{c: c, cols: cols, widths: make([]int, len(cols))}
	for i, col := range cols {
		w.widths[i] = col.Width
		if col.Width == 0 {
			w.widths[i] = displayWidth(col.Name)
		}
	}
	w.unboundedLast = len(cols) > 0 && cols[len(cols)-1].Width == 0
	return w
}

func (w *TableWriter) line(cells []string, upper bool) string {
	var b strings.Builder
	last := len(w.cols) - 1
	for i, col := range w.cols {
		cell := ""
		if i < len(cells) {
			cell = cells[i]
		}
		if upper {
			cell = strings.ToUpper(cell)
		}
		if i == last && w.unboundedLast {
			b.WriteString(cell)
			continue
		}
		if displayWidth(cell) > w.widths[i] {
			cell = ellipsize(cell, w.widths[i])
		}
		pad := strings.Repeat(" ", max(0, w.widths[i]-displayWidth(cell)))
		if col.Align == Right {
			b.WriteString(pad)
			b.WriteString(cell)
		} else {
			b.WriteString(cell)
			b.WriteString(pad)
		}
		if i < last {
			b.WriteString("  ")
		}
	}
	return strings.TrimRight(b.String(), " ") + "\n"
}

// Row prints one row, after the header on the first call; a cell too wide for its column ends in an ellipsis.
func (w *TableWriter) Row(cells ...string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	var out string
	if !w.started {
		w.started = true
		names := make([]string, len(w.cols))
		for i, col := range w.cols {
			names[i] = col.Name
		}
		out = w.line(names, true)
	}
	out += w.line(cells, false)
	w.c.writeData([]byte(out))
}

// Table prints headers and rows to stdout as a table with columns wide enough for every cell.
func (c *Console) Table(headers []string, rows [][]string) {
	maxCols := len(headers)
	for _, r := range rows {
		maxCols = max(maxCols, len(r))
	}
	widths := make([]int, maxCols)
	for i, h := range headers {
		widths[i] = displayWidth(h)
	}
	for _, r := range rows {
		for i, cell := range r {
			widths[i] = max(widths[i], displayWidth(cell))
		}
	}
	cols := make([]Column, maxCols)
	for i := range cols {
		cols[i].Width = widths[i]
		if i < len(headers) {
			cols[i].Name = headers[i]
		}
	}
	w := c.TableWriter(cols...)
	w.unboundedLast = false
	w.mu.Lock()
	w.started = true
	w.mu.Unlock()
	c.writeData([]byte(w.line(headers, true)))
	for _, r := range rows {
		c.writeData([]byte(w.line(r, false)))
	}
}
