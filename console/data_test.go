package console

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

func TestTable(t *testing.T) {
	c, out, _ := newTest(Options{Color: ColorNever})
	c.Table([]string{"package", "version"}, [][]string{
		{"go", "v1.26.5"},
		{"kubectl", "v1.34.1"},
	})
	want := "PACKAGE  VERSION\ngo       v1.26.5\nkubectl  v1.34.1\n"
	if out.String() != want {
		t.Errorf("table:\n%q\nwant:\n%q", out.String(), want)
	}
}

func TestDataGoesToStdoutUnchanged(t *testing.T) {
	c, out, errb := newTest(Options{Color: ColorNever})
	c.Data("%s v%s\n", "go", "1.26.5")
	if got, want := out.String(), "go v1.26.5\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
	if errb.Len() != 0 {
		t.Errorf("data leaked to stderr: %q", errb.String())
	}
}

func TestDataIgnoresQuiet(t *testing.T) {
	c, out, _ := newTest(Options{Quiet: true, Color: ColorNever})
	c.Data("payload\n")
	if out.String() != "payload\n" {
		t.Errorf("quiet suppressed data output: %q", out.String())
	}
}

func TestDataNeverColored(t *testing.T) {
	c, out, _ := newTest(Options{Color: ColorAlways})
	c.Data("plain\n")
	if strings.Contains(out.String(), "\x1b[") {
		t.Errorf("data output contains ANSI codes: %q", out.String())
	}
}

func TestJSONGoesToStdoutIndented(t *testing.T) {
	c, out, errb := newTest(Options{Color: ColorNever})
	if err := c.JSON(map[string]string{"name": "go"}); err != nil {
		t.Fatalf("JSON: %v", err)
	}
	if got, want := out.String(), "{\n  \"name\": \"go\"\n}\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
	if errb.Len() != 0 {
		t.Errorf("json leaked to stderr: %q", errb.String())
	}
}

func TestJSONIgnoresQuiet(t *testing.T) {
	c, out, _ := newTest(Options{Quiet: true, Color: ColorNever})
	if err := c.JSON([]int{1}); err != nil {
		t.Fatalf("JSON: %v", err)
	}
	if out.Len() == 0 {
		t.Errorf("quiet suppressed json output")
	}
}

func TestJSONLineIsCompactWithTrailingNewline(t *testing.T) {
	c, out, _ := newTest(Options{Color: ColorNever})
	if err := c.JSONLine(map[string]any{"name": "jq", "ok": true}); err != nil {
		t.Fatalf("JSONLine: %v", err)
	}
	if got, want := out.String(), "{\"name\":\"jq\",\"ok\":true}\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestBlankWritesNewline(t *testing.T) {
	c, out, _ := newTest(Options{Color: ColorNever})
	c.Blank()
	if out.String() != "\n" {
		t.Errorf("blank = %q", out.String())
	}
}

func TestTableAlignsMultiByteCells(t *testing.T) {
	c, out, _ := newTest(Options{Color: ColorNever})
	c.Table([]string{"name", "note"}, [][]string{{"é", "x"}, {"ab", "y"}})
	if got, want := out.String(), "NAME  NOTE\né     x\nab    y\n"; got != want {
		t.Errorf("table = %q, want %q", got, want)
	}
}

func TestTableHeaderPrintsWithoutRows(t *testing.T) {
	c, out, _ := newTest(Options{Color: ColorNever})
	c.Table([]string{"name"}, nil)
	if got, want := out.String(), "NAME\n"; got != want {
		t.Errorf("table = %q, want %q", got, want)
	}
}

func TestTableWriterHeaderOnFirstRowOnly(t *testing.T) {
	c, out, _ := newTest(Options{Color: ColorNever})
	w := c.TableWriter(Column{Name: "name"})
	if out.Len() != 0 {
		t.Fatalf("header written before any row: %q", out.String())
	}
	w.Row("a")
	w.Row("b")
	if got, want := out.String(), "NAME\na\nb\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestTableWriterRowsFromManyGoroutinesPrintHeaderOnce(t *testing.T) {
	c, out, _ := newTest(Options{Color: ColorNever})
	w := c.TableWriter(Column{Name: "name"})
	var wg sync.WaitGroup
	for g := range 8 {
		wg.Go(func() {
			for i := range 25 {
				w.Row(fmt.Sprintf("row %d-%d", g, i))
			}
		})
	}
	wg.Wait()
	got := out.String()
	if n := strings.Count(got, "NAME\n"); n != 1 {
		t.Errorf("header appears %d times, want 1 in %q", n, got)
	}
	if !strings.HasPrefix(got, "NAME\n") {
		t.Errorf("stdout does not start with the header: %q", got)
	}
	for g := range 8 {
		for i := range 25 {
			if n := strings.Count(got, fmt.Sprintf("row %d-%d\n", g, i)); n != 1 {
				t.Errorf("row %d-%d appears %d times, want 1", g, i, n)
			}
		}
	}
}

func TestTableWriterTruncatesWithEllipsis(t *testing.T) {
	c, out, _ := newTest(Options{Color: ColorNever})
	w := c.TableWriter(Column{Name: "NAME", Width: 6}, Column{Name: "VER"})
	w.Row("kubernetes", "1.2")
	if got, want := out.String(), "NAME    VER\nkuber…  1.2\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestTableWriterRightAlign(t *testing.T) {
	c, out, _ := newTest(Options{Color: ColorNever})
	w := c.TableWriter(Column{Name: "SIZE", Width: 6, Align: Right}, Column{Name: "NAME"})
	w.Row("12 KB", "jq")
	if got, want := out.String(), "  SIZE  NAME\n 12 KB  jq\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestTableWriterLastColumnUnbounded(t *testing.T) {
	c, out, _ := newTest(Options{Color: ColorNever})
	w := c.TableWriter(Column{Name: "N", Width: 2}, Column{Name: "DESCRIPTION"})
	w.Row("a", "a very long description that flows")
	if got, want := out.String(), "N   DESCRIPTION\na   a very long description that flows\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestTableWriterLastColumnBoundedWhenDeclared(t *testing.T) {
	c, out, _ := newTest(Options{Color: ColorNever})
	w := c.TableWriter(Column{Name: "N", Width: 2}, Column{Name: "D", Width: 4})
	w.Row("a", "abcdef")
	if got, want := out.String(), "N   D\na   abc…\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestDataPlaneIsSerializedWithNarration(t *testing.T) {
	c, out, errb := newTest(Options{Color: ColorNever})
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				c.Data("row %d-%d\n", g, i)
				c.Infof("note %d-%d", g, i)
			}
		}()
	}
	wg.Wait()
	for g := 0; g < 8; g++ {
		for i := 0; i < 50; i++ {
			if n := strings.Count(out.String(), fmt.Sprintf("row %d-%d\n", g, i)); n != 1 {
				t.Fatalf("row %d-%d appears %d times", g, i, n)
			}
			if n := strings.Count(errb.String(), fmt.Sprintf("INFO note %d-%d\n", g, i)); n != 1 {
				t.Fatalf("note %d-%d appears %d times", g, i, n)
			}
		}
	}
}
