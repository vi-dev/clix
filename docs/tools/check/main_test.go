package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func write(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

const goSource = `package demo

import "fmt"

func first() {
	fmt.Println("first")
}

// second says hello.
func second(name string) string {
	if name == "" {
		return "nobody" // fallback
	}
	return fmt.Sprintf("hello %s", name)
}
`

const secondPrinted = "func second(name string) string {\n" +
	"    if name == \"\" {\n" +
	"        return \"nobody\"\n" +
	"    }\n" +
	"    return fmt.Sprintf(\"hello %s\", name)\n" +
	"}"

const captureText = "OK Installed jq 1.8.3 (2s)\n---stdout---\njq 1.8.3\n"

func demoRoot(t *testing.T, page string) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, "docs/captures/install-ok.txt", captureText)
	write(t, root, "demo/main.go", goSource)
	write(t, root, "docs/guide.md", page)
	return root
}

func goodPage() string {
	return "# Guide\n\nInstalling prints:\n\n" +
		"<!-- capture: install-ok.txt -->\n```text\n" +
		strings.TrimSuffix(captureText, "\n") + "\n```\n\n" +
		"The code:\n\n<!-- code: demo/main.go#second -->\n```go\n" +
		secondPrinted + "\n```\n\n" +
		"Run it:\n\n```text\n$ go run ./demo\n```\n"
}

func TestMaskRemovesTimeDependentValues(t *testing.T) {
	cases := []struct{ in, want string }{
		{`{"seq":1,"time":"2026-10-20T02:55:02.430627Z","event":"begin"}`, `{"seq":1,"time":"T","event":"begin"}`},
		{`{"state":"done","elapsed_ms":1201}`, `{"state":"done","elapsed_ms":N}`},
		{"OK Installed jq 1.8.3 (2s)", "OK Installed jq 1.8.3"},
		{"OK Built slow 1.0 (1m05s)", "OK Built slow 1.0"},
		{`{"state":"progress","done":3137536,"total":4194304}`, `{"state":"progress","done":N,"total":4194304}`},
		{`{"state":"summary","done":3}`, `{"state":"summary","done":3}`},
		{"OK Checked tool (1 step)", "OK Checked tool (1 step)"},
		{`{"time":"2026-10-20T02:55:02.43Z"}`, `{"time":"2026-10-20T02:55:02.43Z"}`},
		{`{"time":"2026-10-20 02:55:02.430627Z"}`, `{"time":"2026-10-20 02:55:02.430627Z"}`},
	}
	for _, c := range cases {
		if got := mask(c.in); got != c.want {
			t.Errorf("mask(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestCompareCapturesReportsFirstDifference(t *testing.T) {
	committed := t.TempDir()
	fresh := t.TempDir()
	write(t, committed, "same.txt", "OK Installed jq 1.8.3 (2s)\n")
	write(t, fresh, "same.txt", "OK Installed jq 1.8.3 (3s)\n")
	write(t, committed, "changed.txt", "one\ntwo\nthree\n")
	write(t, fresh, "changed.txt", "one\nTWO\nthree\n")
	write(t, committed, "gone.txt", "x\n")
	write(t, fresh, "extra.txt", "y\n")
	write(t, committed, "unchecked/live-80.txt", "frames\n")
	got := compareCaptures(committed, fresh)
	want := []string{
		`capture changed.txt differs at line 2: committed "two", fresh "TWO"`,
		"capture extra.txt is not committed",
		"capture gone.txt was not recorded",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("compareCaptures:\n got %q\nwant %q", got, want)
	}
}

func TestExtractFuncPrintsOneFunction(t *testing.T) {
	root := t.TempDir()
	write(t, root, "main.go", goSource)
	got, err := extractFunc(filepath.Join(root, "main.go"), "second")
	if err != nil {
		t.Fatal(err)
	}
	if got != secondPrinted {
		t.Errorf("extractFunc:\n%s\nwant:\n%s", got, secondPrinted)
	}
	if _, err := extractFunc(filepath.Join(root, "main.go"), "third"); err == nil {
		t.Error("extractFunc found a function that does not exist")
	}
}

func TestVerifyPagesAcceptsMatchingBlocks(t *testing.T) {
	root := demoRoot(t, goodPage())
	write(t, root, "docs/superpowers/notes.md", "```\nunmarked\n```\n")
	write(t, root, "docs/captures/notes.md", "```\nunmarked\n```\n")
	write(t, root, "docs/tools/notes.md", "```\nunmarked\n```\n")
	if got := verifyPages(root); len(got) != 0 {
		t.Errorf("verifyPages reported %q", got)
	}
}

func TestVerifyPagesRejectsOneByteEdit(t *testing.T) {
	page := strings.Replace(goodPage(), "jq 1.8.3\n```", "jq 1.8.4\n```", 1)
	root := demoRoot(t, page)
	want := []string{"docs/guide.md:6: block differs from capture install-ok.txt at line 3"}
	if got := verifyPages(root); !reflect.DeepEqual(got, want) {
		t.Errorf("verifyPages:\n got %q\nwant %q", got, want)
	}
}

func TestVerifyPagesRejectsRenamedFunction(t *testing.T) {
	root := demoRoot(t, goodPage())
	write(t, root, "demo/main.go", strings.Replace(goSource, "func second(", "func greet(", 1))
	want := []string{"docs/guide.md:15: block differs from demo/main.go#second: no function second"}
	if got := verifyPages(root); !reflect.DeepEqual(got, want) {
		t.Errorf("verifyPages:\n got %q\nwant %q", got, want)
	}
}

func TestVerifyPagesRejectsMissingMarker(t *testing.T) {
	page := strings.Replace(goodPage(), "<!-- code: demo/main.go#second -->\n", "", 1)
	page += "\n<!-- capture: nosuch.txt -->\n```text\nx\n```\n"
	root := demoRoot(t, page)
	write(t, root, "README.md", "# Readme\n\n```sh\n$ go run .\n```\n")
	want := []string{
		"README.md:3: missing marker before fenced block",
		"docs/guide.md:14: missing marker before fenced block",
		"docs/guide.md:30: capture nosuch.txt not found",
	}
	if got := verifyPages(root); !reflect.DeepEqual(got, want) {
		t.Errorf("verifyPages:\n got %q\nwant %q", got, want)
	}
}

func TestCheckLinksResolvesFilesAndHeadings(t *testing.T) {
	root := t.TempDir()
	write(t, root, "docs/other.md", "# Other page\n\n## The `--color` flag\n")
	page := "# Guide\n\n## Getting started, quickly\n\n" +
		"See [other](other.md), [flag](other.md#the---color-flag), [here](#getting-started-quickly),\n" +
		"[web](https://example.com/x#y), [mail](mailto:a@example.com), [up](../docs/other.md).\n" +
		"Broken: [gone](missing.md), [anchor](other.md#nowhere), [self](#nowhere).\n" +
		"```text\n[inside](fence.md)\n```\n"
	write(t, root, "docs/guide.md", page)
	got := checkLinks(root, "docs/guide.md", page)
	want := []string{
		"docs/guide.md:7: link missing.md does not resolve",
		"docs/guide.md:7: link other.md#nowhere does not resolve",
		"docs/guide.md:7: link #nowhere does not resolve",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("checkLinks:\n got %q\nwant %q", got, want)
	}
}

func TestVerifyPagesFindsEveryFenceForm(t *testing.T) {
	page := "# Fences\n\n" +
		"~~~\nunmarked tildes\n~~~\n\n" +
		"- A list item:\n\n  ```sh\n  app status\n  ```\n\n" +
		"<!-- capture: install-ok.txt -->\n````text\nOK Installed jq 1.8.3 (2s)\n```\n````\n\n" +
		"<!--\n```text\nhidden [gone](missing.md)\n```\n-->\n\n" +
		"<!-- capture: install-ok.txt -->\n   ```text\n   OK Installed jq 1.8.3 (2s)\n   ---stdout---\n   jq 1.8.3\n   ```\n"
	root := t.TempDir()
	write(t, root, "docs/captures/install-ok.txt", captureText)
	write(t, root, "docs/fences.md", page)
	want := []string{
		"docs/fences.md:3: missing marker before fenced block",
		"docs/fences.md:9: missing marker before fenced block",
		"docs/fences.md:14: block differs from capture install-ok.txt at line 2",
	}
	if got := verifyPages(root); !reflect.DeepEqual(got, want) {
		t.Errorf("verifyPages:\n got %q\nwant %q", got, want)
	}
	blocks := pageBlocks(page)
	if len(blocks) != 4 {
		t.Fatalf("pageBlocks found %d blocks, want 4", len(blocks))
	}
	if want := "OK Installed jq 1.8.3 (2s)\n```"; blocks[2].body != want {
		t.Errorf("four-backtick body = %q, want %q", blocks[2].body, want)
	}
}

func TestCheckLinksSkipsCodeSpans(t *testing.T) {
	root := t.TempDir()
	page := "# Page\n\nWrite `[text](path)` or ``[a](`b`)`` to link; [bad](missing.md) is checked.\n"
	write(t, root, "docs/page.md", page)
	want := []string{"docs/page.md:3: link missing.md does not resolve"}
	if got := checkLinks(root, "docs/page.md", page); !reflect.DeepEqual(got, want) {
		t.Errorf("checkLinks:\n got %q\nwant %q", got, want)
	}
}

func TestCompareCapturesReportsUnreadableDirectory(t *testing.T) {
	committed := t.TempDir()
	write(t, committed, "a.txt", "x\n")
	missing := filepath.Join(t.TempDir(), "nothing")
	got := compareCaptures(committed, missing)
	if len(got) != 2 || !strings.HasPrefix(got[0], "cannot read "+missing) || got[1] != "capture a.txt was not recorded" {
		t.Errorf("compareCaptures = %q", got)
	}
}
