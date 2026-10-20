package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

const usage = `usage: check [-root dir] [-compare] [-verify]

-compare records the compared captures again into a temporary directory and
reports every file that differs from the committed one in docs/captures, after
masking times, durations and progress samples. make docs-captures needs python3
only for the files in docs/captures/unchecked/; docs/tools/capture.sh
--checked-only records the compared set without it.

-verify checks README.md and docs/**/*.md, skipping docs/captures, docs/tools
and docs/superpowers. Every fenced block has a marker on the line before it,
either <!-- capture: <name> --> with <name> relative to docs/captures, or
<!-- code: <path>#<function> --> with <path> relative to the root; a block
with the info string text whose first line starts with "$ " needs none. A
capture block equals the file without its last newline. A code block equals
the function as go/printer prints it with tab indentation, space alignment and
tab width 8, with each leading tab then replaced by four spaces. Every link
without a scheme names an existing path relative to its page, and its anchor
matches a heading of the linked file, or of the same page for a bare #anchor.

Problems go to stdout, one per line; the exit status is 1 when any was found.

`

type block struct {
	line int
	kind string
	ref  string
	body string
}

var (
	timeField    = regexp.MustCompile(`"time":"\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d\.\d{6}Z"`)
	elapsedField = regexp.MustCompile(`"elapsed_ms":\d+`)
	secondsTail  = regexp.MustCompile(` \(\d+s\)`)
	minutesTail  = regexp.MustCompile(` \(\d+m\d{2}s\)`)
	doneField    = regexp.MustCompile(`"done":\d+`)
	marker       = regexp.MustCompile(`^<!-- (capture|code): (\S+) -->$`)
	linkTarget   = regexp.MustCompile(`\]\(([^)\s]*)`)
	scheme       = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*:`)
	heading      = regexp.MustCompile(`^#{1,6}\s+(.*?)\s*#*\s*$`)
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	flags := flag.NewFlagSet("check", flag.ContinueOnError)
	flags.Usage = func() {
		say(flags.Output(), usage)
		flags.PrintDefaults()
	}
	root := flags.String("root", ".", "the repository root")
	compare := flags.Bool("compare", false, "compare fresh captures with the committed ones")
	verify := flags.Bool("verify", false, "verify the blocks and links of the pages")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if !*compare && !*verify {
		flags.Usage()
		return 2
	}
	var problems []string
	if *compare {
		found, err := freshCompare(*root)
		if err != nil {
			say(os.Stderr, "check: "+err.Error()+"\n")
			return 1
		}
		problems = append(problems, found...)
	}
	if *verify {
		problems = append(problems, verifyPages(*root)...)
	}
	for _, p := range problems {
		say(os.Stdout, p+"\n")
	}
	if len(problems) > 0 {
		return 1
	}
	return 0
}

func say(w io.Writer, s string) {
	_, _ = io.WriteString(w, s)
}

func freshCompare(root string) ([]string, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	tmp, err := os.MkdirTemp("", "clix-captures-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	cmd := exec.Command(filepath.Join(abs, "docs", "tools", "capture.sh"), "--checked-only", tmp)
	cmd.Dir = abs
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("capture.sh: %w", err)
	}
	return compareCaptures(filepath.Join(abs, "docs", "captures"), tmp), nil
}

func mask(s string) string {
	s = timeField.ReplaceAllString(s, `"time":"T"`)
	s = elapsedField.ReplaceAllString(s, `"elapsed_ms":N`)
	s = secondsTail.ReplaceAllString(s, "")
	s = minutesTail.ReplaceAllString(s, "")
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if strings.Contains(line, `"state":"progress"`) {
			lines[i] = doneField.ReplaceAllString(line, `"done":N`)
		}
	}
	return strings.Join(lines, "\n")
}

func compareCaptures(committed, fresh string) []string {
	old, problems := captureFiles(committed)
	cur, more := captureFiles(fresh)
	problems = append(problems, more...)
	names := map[string]bool{}
	for name := range old {
		names[name] = true
	}
	for name := range cur {
		names[name] = true
	}
	sorted := make([]string, 0, len(names))
	for name := range names {
		sorted = append(sorted, name)
	}
	sort.Strings(sorted)
	for _, name := range sorted {
		a, inOld := old[name]
		b, inCur := cur[name]
		switch {
		case !inOld:
			problems = append(problems, fmt.Sprintf("capture %s is not committed", name))
		case !inCur:
			problems = append(problems, fmt.Sprintf("capture %s was not recorded", name))
		default:
			want := strings.Split(mask(a), "\n")
			got := strings.Split(mask(b), "\n")
			if k := firstDifference(want, got); k > 0 {
				problems = append(problems, fmt.Sprintf("capture %s differs at line %d: committed %q, fresh %q",
					name, k, lineAt(want, k), lineAt(got, k)))
			}
		}
	}
	return problems
}

func captureFiles(dir string) (map[string]string, []string) {
	files := map[string]string{}
	var problems []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			problems = append(problems, fmt.Sprintf("cannot read %s: %v", path, err))
			return nil
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, "unchecked/") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			problems = append(problems, fmt.Sprintf("cannot read %s: %v", path, err))
			return nil
		}
		files[rel] = string(data)
		return nil
	})
	if err != nil {
		problems = append(problems, fmt.Sprintf("cannot read %s: %v", dir, err))
	}
	return files, problems
}

func firstDifference(a, b []string) int {
	for i := 0; i < len(a) || i < len(b); i++ {
		if i >= len(a) || i >= len(b) || a[i] != b[i] {
			return i + 1
		}
	}
	return 0
}

func lineAt(lines []string, k int) string {
	if k > len(lines) {
		return ""
	}
	return lines[k-1]
}

func verifyPages(root string) []string {
	var pages []string
	if _, err := os.Stat(filepath.Join(root, "README.md")); err == nil {
		pages = append(pages, "README.md")
	}
	_ = filepath.WalkDir(filepath.Join(root, "docs"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if rel == "docs/captures" || rel == "docs/tools" || rel == "docs/superpowers" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(rel, ".md") {
			pages = append(pages, rel)
		}
		return nil
	})
	var problems []string
	for _, page := range pages {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(page)))
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", page, err))
			continue
		}
		md := string(data)
		for _, b := range pageBlocks(md) {
			if p := checkBlock(root, b); p != "" {
				problems = append(problems, fmt.Sprintf("%s:%d: %s", page, b.line, p))
			}
		}
		problems = append(problems, checkLinks(root, page, md)...)
	}
	return problems
}

func checkBlock(root string, b block) string {
	switch b.kind {
	case "shell":
		return ""
	case "capture":
		data, err := os.ReadFile(filepath.Join(root, "docs", "captures", filepath.FromSlash(b.ref)))
		if err != nil {
			return fmt.Sprintf("capture %s not found", b.ref)
		}
		want := strings.TrimSuffix(string(data), "\n")
		if want != b.body {
			k := firstDifference(strings.Split(want, "\n"), strings.Split(b.body, "\n"))
			return fmt.Sprintf("block differs from capture %s at line %d", b.ref, k)
		}
		return ""
	case "code":
		path, name, _ := strings.Cut(b.ref, "#")
		want, err := extractFunc(filepath.Join(root, filepath.FromSlash(path)), name)
		if err != nil {
			return fmt.Sprintf("block differs from %s: %v", b.ref, err)
		}
		if want != b.body {
			return fmt.Sprintf("block differs from %s", b.ref)
		}
		return ""
	}
	return "missing marker before fenced block"
}

func extractFunc(path, name string) (string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return "", err
	}
	recv, fn, isMethod := strings.Cut(name, ".")
	if !isMethod {
		fn, recv = recv, ""
	}
	for _, decl := range file.Decls {
		f, ok := decl.(*ast.FuncDecl)
		if !ok || f.Name.Name != fn || receiverName(f) != recv {
			continue
		}
		var buf bytes.Buffer
		cfg := printer.Config{Mode: printer.UseSpaces | printer.TabIndent, Tabwidth: 8}
		if err := cfg.Fprint(&buf, fset, f); err != nil {
			return "", err
		}
		lines := strings.Split(buf.String(), "\n")
		for i, line := range lines {
			trimmed := strings.TrimLeft(line, "\t")
			lines[i] = strings.Repeat("    ", len(line)-len(trimmed)) + trimmed
		}
		return strings.TrimSuffix(strings.Join(lines, "\n"), "\n"), nil
	}
	return "", fmt.Errorf("no function %s", name)
}

func receiverName(f *ast.FuncDecl) string {
	if f.Recv == nil || len(f.Recv.List) == 0 {
		return ""
	}
	t := f.Recv.List[0].Type
	if star, ok := t.(*ast.StarExpr); ok {
		t = star.X
	}
	switch v := t.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.IndexExpr:
		if id, ok := v.X.(*ast.Ident); ok {
			return id.Name
		}
	case *ast.IndexListExpr:
		if id, ok := v.X.(*ast.Ident); ok {
			return id.Name
		}
	}
	return ""
}

type fence struct {
	open   int
	close  int
	indent int
	info   string
}

func fenceOpen(line string) (char byte, size, indent int, info string, ok bool) {
	for indent < 3 && indent < len(line) && line[indent] == ' ' {
		indent++
	}
	rest := line[indent:]
	if rest == "" || (rest[0] != '`' && rest[0] != '~') {
		return 0, 0, 0, "", false
	}
	char = rest[0]
	for size < len(rest) && rest[size] == char {
		size++
	}
	if size < 3 {
		return 0, 0, 0, "", false
	}
	info = strings.TrimSpace(rest[size:])
	if char == '`' && strings.Contains(info, "`") {
		return 0, 0, 0, "", false
	}
	return char, size, indent, info, true
}

func fenceCloses(line string, char byte, size int) bool {
	trimmed := strings.TrimLeft(line, " ")
	if len(line)-len(trimmed) > 3 {
		return false
	}
	n := 0
	for n < len(trimmed) && trimmed[n] == char {
		n++
	}
	return n >= size && strings.TrimSpace(trimmed[n:]) == ""
}

func layout(lines []string) ([]fence, []bool) {
	var fences []fence
	prose := make([]bool, len(lines))
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if start := strings.Index(line, "<!--"); start >= 0 && !strings.Contains(line[start:], "-->") {
			for i < len(lines) && !strings.Contains(lines[i], "-->") {
				i++
			}
			continue
		}
		char, size, indent, info, ok := fenceOpen(line)
		if !ok {
			prose[i] = true
			continue
		}
		end := i + 1
		for end < len(lines) && !fenceCloses(lines[end], char, size) {
			end++
		}
		fences = append(fences, fence{open: i, close: end, indent: indent, info: info})
		i = end
	}
	return fences, prose
}

func pageBlocks(md string) []block {
	lines := strings.Split(md, "\n")
	fences, _ := layout(lines)
	blocks := make([]block, 0, len(fences))
	for _, f := range fences {
		body := make([]string, 0, f.close-f.open)
		for _, line := range lines[f.open+1 : min(f.close, len(lines))] {
			trimmed := strings.TrimLeft(line, " ")
			cut := min(len(line)-len(trimmed), f.indent)
			body = append(body, line[cut:])
		}
		b := block{line: f.open + 1, body: strings.Join(body, "\n")}
		if f.open > 0 {
			if m := marker.FindStringSubmatch(strings.TrimSpace(lines[f.open-1])); m != nil {
				b.kind, b.ref = m[1], m[2]
			}
		}
		if b.kind == "" && f.info == "text" && len(body) > 0 && strings.HasPrefix(body[0], "$ ") {
			b.kind = "shell"
		}
		blocks = append(blocks, b)
	}
	return blocks
}

func withoutCodeSpans(line string) string {
	var b strings.Builder
	for i := 0; i < len(line); {
		if line[i] != '`' {
			b.WriteByte(line[i])
			i++
			continue
		}
		n := 0
		for i+n < len(line) && line[i+n] == '`' {
			n++
		}
		end := -1
		for j := i + n; j < len(line); {
			if line[j] != '`' {
				j++
				continue
			}
			m := 0
			for j+m < len(line) && line[j+m] == '`' {
				m++
			}
			if m == n {
				end = j + m
				break
			}
			j += m
		}
		if end < 0 {
			b.WriteString(line[i : i+n])
			i += n
			continue
		}
		i = end
	}
	return b.String()
}

func checkLinks(root, page, md string) []string {
	var problems []string
	dir := filepath.Dir(filepath.Join(root, filepath.FromSlash(page)))
	self := filepath.Join(root, filepath.FromSlash(page))
	lines := strings.Split(md, "\n")
	_, prose := layout(lines)
	for i, line := range lines {
		if !prose[i] {
			continue
		}
		for _, m := range linkTarget.FindAllStringSubmatch(withoutCodeSpans(line), -1) {
			target := m[1]
			if scheme.MatchString(target) {
				continue
			}
			if !linkResolves(dir, self, target) {
				problems = append(problems, fmt.Sprintf("%s:%d: link %s does not resolve", page, i+1, target))
			}
		}
	}
	return problems
}

func linkResolves(dir, self, target string) bool {
	path, anchor, _ := strings.Cut(target, "#")
	file := self
	if path != "" {
		file = filepath.Join(dir, filepath.FromSlash(path))
	} else if anchor == "" {
		return false
	}
	if _, err := os.Stat(file); err != nil {
		return false
	}
	if anchor == "" {
		return true
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return false
	}
	for _, h := range headings(string(data)) {
		if slug(h) == anchor {
			return true
		}
	}
	return false
}

func headings(md string) []string {
	var found []string
	lines := strings.Split(md, "\n")
	_, prose := layout(lines)
	for i, line := range lines {
		if !prose[i] {
			continue
		}
		if m := heading.FindStringSubmatch(line); m != nil {
			found = append(found, m[1])
		}
	}
	return found
}

func slug(h string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(h) {
		switch {
		case r == ' ':
			b.WriteRune('-')
		case unicode.IsLetter(r), unicode.IsDigit(r), r == '-', r == '_':
			b.WriteRune(r)
		}
	}
	return b.String()
}
