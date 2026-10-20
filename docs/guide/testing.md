# Testing

This page is for a developer writing tests for commands built on clix.
Afterwards you can check which lines and tasks a function produced, pin
the exact text a user sees, read the JSON events of a run, and run a
whole command with its flags and exit code.

The `consoletest` package records narration without a terminal, and
`console.ReadEvents` reads a JSON stream back. The tests below are in
[examples/app/main_test.go](../../examples/app/main_test.go) and run
with `go test ./examples/app/`.

## Record what a function narrates

`consoletest.Context(ctx)` returns a context holding a recording
`Narrator`, and the narrator itself. Code that takes its narrator from the
context, as `checkPackages` does with `console.FromContext`, runs against
it unchanged.

<!-- code: examples/app/main_test.go#TestCheckRecordsOutcomes -->
```go
func TestCheckRecordsOutcomes(t *testing.T) {
    ctx, n := consoletest.Context(t.Context())
    cmd := &cobra.Command{}
    cmd.SetContext(ctx)
    err := checkPackages(cmd, []string{"tool", "broken"})
    if _, ok := errors.AsType[*clix.ExitError](err); !ok {
        t.Fatalf("err = %v, want an exit error", err)
    }
    if done, _, outcome := n.TaskFor("Checking tool").Snapshot(); !done || outcome != "Checked tool (1 step)" {
        t.Errorf("tool: done %v, outcome %q", done, outcome)
    }
    broken := n.TaskFor("Checking broken")
    if _, failed, outcome := broken.Snapshot(); !failed || outcome != "Failed to check broken" {
        t.Errorf("broken: failed %v, outcome %q", failed, outcome)
    }
    if got, want := broken.Phases(), []string{"testing"}; !slices.Equal(got, want) {
        t.Errorf("broken phases = %q, want %q", got, want)
    }
    if got, want := broken.Warns(), []string{`broken: test step 1 ("false"): exit status 1`}; !slices.Equal(got, want) {
        t.Errorf("broken warns = %q, want %q", got, want)
    }
    if got, want := n.Successes(), []string{"Checked 1 package, 1 failed"}; !slices.Equal(got, want) {
        t.Errorf("successes = %q, want %q", got, want)
    }
    if got, want := n.Hints(), []string{"Retry with `app check broken`"}; !slices.Equal(got, want) {
        t.Errorf("hints = %q, want %q", got, want)
    }
}
```

The narrator keeps the text of each line, without its prefix, in
`Infos`, `Successes`, `Warns` and `Hints`; Debug lines are not kept.
`TaskFor(label)` finds a task by its label. A task keeps the lines
written through it, so the broken item's warning is on its task.
`Snapshot` returns whether it ended done or failed and with which text,
and `Phases` the phases it went through. `ProgressCalls`, `Discarded`
and `Output` return the rest of what the task saw.

## Pin the exact text

`consoletest.Transcript` runs a function against a real console and
returns everything it wrote to stderr. Its clock stands still and moves
100 ms only when the function calls `tick`, so times and live frames come
out the same on every run.

<!-- code: examples/app/main_test.go#TestInstallTranscript -->
```go
func TestInstallTranscript(t *testing.T) {
    got := consoletest.Transcript(t, console.Options{Color: console.ColorNever}, func(c *console.Console, tick func()) {
        if err := install(console.NewContext(t.Context(), c), "jq", "1.8.3"); err != nil {
            t.Fatal(err)
        }
    })
    if want := "OK Installed jq 1.8.3\n"; got != want {
        t.Errorf("transcript:\n got %q\nwant %q", got, want)
    }
}
```

The real run in [tasks](tasks.md#start-a-task-and-end-it) prints
`(2s)` after the outcome; here the clock never moved, so the line has no
time. `console.NewContext` puts the console in the context that `install`
reads.

## Check the JSON events

Run with `--narration json` and read stderr back with
`console.ReadEvents`. It skips a line that does not start with `{` and
returns an error for one that does but does not decode.

<!-- code: examples/app/main_test.go#TestInstallEvents -->
```go
func TestInstallEvents(t *testing.T) {
    var stdout, stderr bytes.Buffer
    root := newRoot()
    root.SetOut(&stdout)
    root.SetErr(&stderr)
    root.SetArgs([]string{"install", "jq@1.8.3", "--narration", "json"})
    if code, err := clix.Execute(t.Context(), root, clix.Options{App: "app", Version: "0.1.0"}); code != 0 {
        t.Fatalf("code, err = %d, %v; want 0", code, err)
    }
    events, err := console.ReadEvents(&stderr)
    if err != nil {
        t.Fatal(err)
    }
    var kinds []string
    for _, e := range events {
        if e.State != "progress" {
            kinds = append(kinds, strings.TrimSpace(e.Event+" "+e.State))
        }
    }
    want := []string{"begin", "task started", "task phase", "task phase", "task done", "exit"}
    if !slices.Equal(kinds, want) {
        t.Errorf("events = %q, want %q", kinds, want)
    }
    if stdout.Len() != 0 {
        t.Errorf("stdout = %q, want empty", stdout.String())
    }
}
```

Progress samples depend on how long the run takes, so the test leaves
them out and checks the rest in order. The fields of each event are in
[JSON events](../reference/json-events.md).

## Run a whole command

`clix.Execute` runs a root command as `clix.Run` does and returns the
exit code and the error instead of exiting, with no signal handling.
`SetOut`, `SetErr` and `SetArgs` on the root give it buffers and
arguments.

<!-- code: examples/app/main_test.go#TestBadColorExitsTwo -->
```go
func TestBadColorExitsTwo(t *testing.T) {
    var stderr bytes.Buffer
    root := newRoot()
    root.SetErr(&stderr)
    root.SetArgs([]string{"--color", "bogus", "status"})
    if code, _ := clix.Execute(t.Context(), root, clix.Options{App: "app", Version: "0.1.0"}); code != 2 {
        t.Fatalf("code = %d, want 2", code)
    }
    if got, want := stderr.String(), "Error: invalid --color value \"bogus\" (want auto, always, or never)\n"; got != want {
        t.Errorf("stderr = %q, want %q", got, want)
    }
}
```

It matches what a user sees from `app --color bogus status`:

<!-- capture: bad-color.txt -->
```text
Error: invalid --color value "bogus" (want auto, always, or never)
---exit 2---
```

Each test builds a fresh root with `newRoot()`, because clix adds its
flags to the root it runs and a second run of the same root panics on a
flag defined twice. Where commands share state, such as flag variables at
package level, the tests do not call `t.Parallel()`.

## Pin plain text, not colour

Tests compare plain lines such as `OK Installed jq 1.8.3`. Pass
`console.ColorNever` to `Transcript`, which reads no environment. A
command run through `Execute` or `Run` reads `FORCE_COLOR`, which turns
colour on under `auto` even though a buffer is never a terminal, so pass
it `--color never` or clear the variable with
`t.Setenv("FORCE_COLOR", "")`. The glyphs and colour codes belong to clix
and its own tests pin them. A test of a command that pins them breaks
when a theme changes and catches nothing about the command.

## Where to go next

- [Signals and exit](signals-and-exit.md): exit codes and what a
  cancelled run prints.
- [JSON events](../reference/json-events.md): every event and field.
- [Flags and environment](../reference/flags-and-environment.md): the
  flags a test can pass.
