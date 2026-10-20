package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/vi-dev/clix"
	"github.com/vi-dev/clix/console"
)

var (
	errNotFound = errors.New("not found")
	errChecksum = errors.New("checksum mismatch")
)

type hintedError struct {
	err  error
	hint string
}

func (e *hintedError) Error() string { return e.err.Error() }

func (e *hintedError) Unwrap() error { return e.err }

func hint(err error) string {
	if h, ok := errors.AsType[*hintedError](err); ok {
		return h.hint
	}
	if errors.Is(err, errNotFound) {
		return "Check the package name or run `app update`"
	}
	return ""
}

var (
	publishDryRun bool
	cleanYes      bool
	cleanDryRun   bool
	statusOutput  *string
	searchOutput  *string
)

func main() {
	clix.Run(newRoot(), clix.Options{App: "app", Version: "0.1.0", EnvPrefix: "APP", Hint: hint})
}

func newRoot() *cobra.Command {
	root := &cobra.Command{
		Use:   "app",
		Short: "Install and check packages",
		Long:  "app installs packages from catalogs into the current project.",
	}
	root.AddCommand(
		installCommand(),
		&cobra.Command{Use: "update", Short: "Update the catalogs", Args: cobra.NoArgs, RunE: narrateUpdate},
		&cobra.Command{Use: "check [package...]", Short: "Check packages (default: every package)", RunE: checkPackages},
		&cobra.Command{Use: "build <name>@<version>", Short: "Build a package from source", Args: cobra.ExactArgs(1), RunE: buildPackage},
		&cobra.Command{Use: "sync", Short: "Install the locked packages", Args: cobra.NoArgs, RunE: syncPackages},
		publishCommand(),
		statusCommand(),
		searchCommand(),
		infoCommand(),
		&cobra.Command{Use: "env", Short: "Print the shell environment", Args: cobra.NoArgs, RunE: printEnv},
		cleanCommand(),
		&cobra.Command{Use: "run -- <command> [args...]", Short: "Run a command in the package environment", Args: cobra.MinimumNArgs(1), RunE: runPassthrough},
	)
	return root
}

func installCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "install <name>@<version>",
		Short: "Install a package",
		Long: "Install resolves a package in the configured catalogs, downloads it,\n" +
			"and adds it to the project. It writes the package under the app home\n" +
			"and records the exact version.",
		Example: "  app install jq               # newest version in the first catalog\n" +
			"  app install jq@1.8.3         # an exact version",
		Args: cobra.ExactArgs(1),
		RunE: installPackage,
	}
}

func publishCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "publish <ref>", Short: "Publish the catalog to a registry", Args: cobra.ExactArgs(1), RunE: publishCatalog}
	cmd.Flags().BoolVar(&publishDryRun, "dry-run", false, "show the plan without pushing")
	return cmd
}

func statusCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "status", Short: "Show the project packages", Args: cobra.NoArgs, RunE: printStatus}
	statusOutput = clix.OutputFlag(cmd)
	return cmd
}

func searchCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "search <term>", Short: "Search the catalogs", Args: cobra.ExactArgs(1), RunE: searchPackages}
	searchOutput = clix.OutputFlag(cmd)
	return cmd
}

func infoCommand() *cobra.Command {
	return &cobra.Command{Use: "info <name>", Short: "Show a package", Args: cobra.ExactArgs(1), RunE: printInfo}
}

func cleanCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "clean", Short: "Remove unused package versions", Args: cobra.NoArgs, RunE: cleanVersions}
	cmd.Flags().BoolVar(&cleanYes, "yes", false, "remove without asking")
	cmd.Flags().BoolVar(&cleanDryRun, "dry-run", false, "show what would be removed")
	return cmd
}

func pause(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}

type slowReader struct {
	ctx   context.Context
	left  int
	chunk int
	wait  time.Duration
	read  int
}

func (r *slowReader) Read(p []byte) (int, error) {
	if r.left == 0 {
		return 0, io.EOF
	}
	n := min(r.chunk-r.read, r.left, len(p))
	r.left -= n
	r.read += n
	if r.read == r.chunk {
		r.read = 0
		if err := pause(r.ctx, r.wait); err != nil {
			return 0, err
		}
	}
	return n, nil
}

func installPackage(cmd *cobra.Command, args []string) error {
	name, version, _ := strings.Cut(args[0], "@")
	if name == "nosuch" {
		return fmt.Errorf("package %s %w", name, errNotFound)
	}
	err := install(cmd.Context(), name, version)
	if errors.Is(err, errChecksum) {
		return &hintedError{err: err, hint: "Run `app install " + args[0] + " --verbose`"}
	}
	return err
}

func install(ctx context.Context, name, version string) error {
	label := name + " " + version
	t := console.FromContext(ctx).Start("Installing " + label)
	o := console.Outcomes{Done: "Installed " + label, Failed: "Failed to install " + label, Cancelled: "Cancelled " + label}
	t.Phase("downloading")
	if name == "zlib" && version == "9.9" {
		return t.End(ctx, errors.New("download zlib 9.9: 404 Not Found"), o)
	}
	size := 4 << 20
	body := &slowReader{ctx: ctx, left: size, chunk: 1 << 20, wait: 375 * time.Millisecond}
	if _, err := io.Copy(io.Discard, console.ProgressReader(t, body, int64(size))); err != nil {
		return t.End(ctx, err, o)
	}
	t.Phase("extracting")
	if err := pause(ctx, 100*time.Millisecond); err != nil {
		return t.End(ctx, err, o)
	}
	if name == "corrupt" {
		return t.End(ctx, fmt.Errorf("%w\nwant sha256:9f86d081884c7d65\ngot  sha256:60303ae22b998861", errChecksum), o)
	}
	return t.End(ctx, nil, o)
}

func narrateUpdate(cmd *cobra.Command, args []string) error {
	c := clix.ConsoleOf(cmd)
	if err := updateCatalog(cmd.Context(), "main"); err != nil {
		return err
	}
	c.Debug("Skipped jq 1.8.3 (unchanged)")
	stale, days := "extra", 9
	c.Warnf("Catalog %s is %d days old — run `app update %s`", stale, days, stale)
	c.Info("Release notes: https://registry.example/notes")
	return nil
}

func updateCatalog(ctx context.Context, name string) error {
	label := "catalog " + name
	o := console.Outcomes{Done: "Updated " + label, Failed: "Failed to update " + label, Cancelled: "Cancelled updating " + label}
	return console.RunTask(ctx, "Updating "+label, "pulling index", o, func(progress console.ProgressFunc) error {
		files := 2
		for i := range files {
			if err := pause(ctx, 25*time.Millisecond); err != nil {
				return err
			}
			progress(int64(i+1), int64(files), console.Items)
		}
		return nil
	})
}

type checkCounts struct {
	checked, failed, upToDate int
	failedNames               []string
}

func checkPackages(cmd *cobra.Command, args []string) error {
	c := console.FromContext(cmd.Context())
	explicit := len(args) > 0
	names := args
	if !explicit {
		names = []string{"broken", "jq", "tool"}
	}
	var n checkCounts
	for _, name := range names {
		checkPackage(c, name, explicit, &n)
	}
	parts := []string{}
	if n.checked > 0 {
		parts = append(parts, "Checked "+console.Plural(n.checked, "package"))
	}
	if n.failed > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", n.failed))
	}
	if n.upToDate > 0 {
		parts = append(parts, fmt.Sprintf("%d up to date", n.upToDate))
	}
	if len(parts) == 0 {
		parts = append(parts, "Nothing to check")
	}
	c.Success(strings.Join(parts, ", "))
	if n.failed > 0 {
		c.Hintf("Retry with `app check %s`", strings.Join(n.failedNames, " "))
		return &clix.ExitError{Code: 1}
	}
	return nil
}

func checkPackage(c console.Narrator, name string, explicit bool, n *checkCounts) {
	t := c.Start("Checking " + name)
	t.Phase("testing")
	switch {
	case name == "broken":
		t.Warnf("%s: test step 1 (\"false\"): exit status 1", name)
		t.Fail("Failed to check " + name)
		n.failed++
		n.failedNames = append(n.failedNames, name)
	case name == "jq" && explicit:
		t.Done("jq up to date (1.8.2)")
		n.upToDate++
	case name == "jq":
		t.Discard()
		c.Debugf("Skipped %s (up to date)", name)
		n.upToDate++
	default:
		t.Done("Checked " + name + " (1 step)")
		n.checked++
	}
}

func buildPackage(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	name, version, _ := strings.Cut(args[0], "@")
	label := name + " " + version
	t := clix.ConsoleOf(cmd).Start("Building " + label)
	o := console.Outcomes{Done: "Built " + label, Failed: "Failed to build " + label, Cancelled: "Cancelled " + label}
	t.Phase("compiling")
	var err error
	switch name {
	case "slow":
		err = compileSlowly(ctx, t)
	case "broken":
		if err = runStep(ctx, t, "printf 'checking for cc... no\\n'; exit 1"); err != nil {
			err = &hintedError{err: fmt.Errorf("configure failed: %w\nsee config.log", err), hint: "Run `app build " + args[0] + " --verbose`"}
		}
	default:
		err = runStep(ctx, t, "echo 'checking for cc... cc'; echo 'cc -O2 -c adler32.c'")
	}
	return t.End(ctx, err, o)
}

func runStep(ctx context.Context, t console.Task, script string) error {
	step := exec.CommandContext(ctx, "sh", "-c", script)
	step.Stdout = t.ErrOut()
	step.Stderr = t.ErrOut()
	return step.Run()
}

func compileSlowly(ctx context.Context, t console.Task) error {
	files := 2
	t.Progress(0, int64(files), console.Items)
	for i := range files {
		if err := pause(ctx, 700*time.Millisecond); err != nil {
			return err
		}
		t.Progress(int64(i+1), int64(files), console.Items)
	}
	return nil
}

func syncPackages(cmd *cobra.Command, args []string) error {
	c := clix.ConsoleOf(cmd)
	locked := [][2]string{{"jq", "1.8.3"}, {"zlib", "1.3.1"}}
	for _, p := range locked {
		if err := install(cmd.Context(), p[0], p[1]); err != nil {
			return err
		}
	}
	c.Successf("Synced %s", console.Plural(len(locked), "package"))
	return nil
}

func publishCatalog(cmd *cobra.Command, args []string) error {
	c := clix.ConsoleOf(cmd)
	ref := args[0]
	t := c.Start("Comparing with " + ref)
	t.Phase("comparing")
	for i := range 42 {
		t.Progress(int64(i+1), 42, console.Items)
	}
	t.Done("Compared with " + ref + ": 1 to push, 41 unchanged")
	if publishDryRun {
		c.Table([]string{"package", "version", "status"}, [][]string{{"jq", "1.8.3", "updated"}})
		c.Successf("Would publish %s: 1 to push, 41 unchanged", ref)
		return nil
	}
	if err := pushPackage(cmd.Context(), c, "jq 1.8.3"); err != nil {
		return err
	}
	c.Successf("Published %s: 1 pushed, 41 unchanged", ref)
	return nil
}

func pushPackage(ctx context.Context, c *console.Console, label string) error {
	t := c.Start("Pushing " + label)
	t.Phase("pushing")
	size := 3 << 20
	body := &slowReader{ctx: ctx, left: size, chunk: 1 << 20, wait: 200 * time.Millisecond}
	_, err := io.Copy(console.ProgressWriter(t, int64(size)), body)
	return t.End(ctx, err, console.Outcomes{Done: "Pushed " + label, Failed: "Failed to push " + label, Cancelled: "Cancelled " + label})
}

func printStatus(cmd *cobra.Command, args []string) error {
	c := clix.ConsoleOf(cmd)
	type row struct {
		Name      string `json:"name"`
		Version   string `json:"version"`
		Installed bool   `json:"installed"`
		Catalog   string `json:"catalog,omitempty"`
	}
	rows := []row{{"jq", "1.8.3", true, "main"}, {"zlib", "1.3.1", false, ""}}
	if *statusOutput == "json" {
		return c.JSON(rows)
	}
	cells := [][]string{}
	for _, r := range rows {
		installed, catalog := "no", r.Catalog
		if r.Installed {
			installed = "yes"
		}
		if catalog == "" {
			catalog = "-"
		}
		cells = append(cells, []string{r.Name, r.Version, catalog, installed})
	}
	c.Table([]string{"package", "version", "catalog", "installed"}, cells)
	c.BlankErr()
	c.Warn("1 package not installed — run `app sync`")
	return nil
}

type catalogPackage struct {
	Name    string `json:"name"`
	Size    string `json:"size"`
	Summary string `json:"summary"`
}

var catalogPackages = []catalogPackage{
	{"jq", "1 MB", "Command-line JSON processor"},
	{"jless", "2 MB", "Command-line JSON viewer"},
	{"jsonnet-bundler", "3 MB", "Package manager for Jsonnet"},
	{"zlib", "512 KB", "Compression library"},
}

func searchPackages(cmd *cobra.Command, args []string) error {
	c := clix.ConsoleOf(cmd)
	w := c.TableWriter(
		console.Column{Name: "package", Width: 8},
		console.Column{Name: "size", Width: 6, Align: console.Right},
		console.Column{Name: "summary"},
	)
	found := 0
	for _, p := range catalogPackages {
		if !strings.Contains(p.Name, args[0]) {
			continue
		}
		found++
		if *searchOutput == "json" {
			if err := c.JSONLine(p); err != nil {
				return err
			}
			continue
		}
		w.Row(p.Name, p.Size, p.Summary)
	}
	if found == 0 {
		c.Warnf("No packages match %q", args[0])
	}
	return nil
}

func printInfo(cmd *cobra.Command, args []string) error {
	c := clix.ConsoleOf(cmd)
	fields := [][2]string{
		{"name", args[0]},
		{"version", "1.8.3"},
		{"homepage", "https://jqlang.org"},
		{"license", ""},
		{"catalog", "main"},
	}
	for _, f := range fields {
		if f[1] != "" {
			c.Data("%-12s%s\n", f[0]+":", f[1])
		}
	}
	return nil
}

func printEnv(cmd *cobra.Command, args []string) error {
	c := clix.ConsoleOf(cmd)
	c.Data("export PATH=\"$HOME/.app/packages/jq/1.8.3/bin:$PATH\"\n")
	c.Data("export APP_ACTIVE=1\n")
	return nil
}

func cleanVersions(cmd *cobra.Command, args []string) error {
	c := clix.ConsoleOf(cmd)
	ctx := cmd.Context()
	c.Table([]string{"package", "version", "size"}, [][]string{{"jq", "1.7.1", "1 MB"}, {"zlib", "1.3", "512 KB"}})
	if cleanDryRun {
		c.Successf("Would remove %s", console.Plural(2, "package version"))
		return nil
	}
	if !cleanYes {
		yes, err := confirm(cmd, "Remove these package versions? [y/N]: ")
		if err != nil {
			return err
		}
		if !yes {
			c.Success("Nothing to remove")
			return nil
		}
	}
	t := c.Start("Removing 2 package versions")
	t.Phase("removing")
	o := console.Outcomes{Done: "Removed 2 package versions", Failed: "Failed to remove 2 package versions", Cancelled: "Cancelled removing 2 package versions"}
	return t.End(ctx, pause(ctx, 50*time.Millisecond), o)
}

func confirm(cmd *cobra.Command, question string) (bool, error) {
	c := clix.ConsoleOf(cmd)
	c.BlankErr()
	c.Prompt(question)
	line, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	answer := strings.TrimSpace(line)
	c.Answer(answer)
	return answer == "y", nil
}

func runPassthrough(cmd *cobra.Command, args []string) error {
	child := exec.CommandContext(cmd.Context(), args[0], args[1:]...)
	child.Stdin = os.Stdin
	child.Stdout = os.Stdout
	child.Stderr = os.Stderr
	err := child.Run()
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return &clix.ExitError{Code: exitErr.ExitCode()}
	}
	return err
}
