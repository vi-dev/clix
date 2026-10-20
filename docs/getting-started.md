# Getting started

This page is for a Go developer writing a first clix program. You leave
with a CLI that narrates, runs a task, fails with a hint, and has a test.
The program is `hello`, with one command, `hello greet <name>`. The full
file with its imports is [examples/hello/main.go](../examples/hello/main.go).

## Create the root command

The commands are plain commands of cobra, the Go command-line library.

<!-- code: examples/hello/main.go#newRoot -->
```go
func newRoot() *cobra.Command {
    root := &cobra.Command{Use: "hello", Short: "Greet people"}
    root.AddCommand(&cobra.Command{
        Use:   "greet <name>",
        Short: "Greet someone by name",
        Args:  cobra.ExactArgs(1),
        RunE:  greet,
    })
    return root
}
```

## Narrate from a command

The command talks to the user through the console that clix builds. Here
it sits in `var con *console.Console` at package level, set by `Setup` in
`main`. `con.Infof` writes an `INFO` line to stderr.

<!-- code: examples/hello/main.go#greet -->
```go
func greet(cmd *cobra.Command, args []string) error {
    name := args[0]
    if name == "nobody" {
        return greetError(name)
    }
    con.Infof("Hello, %s", name)
    return greetTask(cmd.Context(), name)
}
```

## Run a task

A task is a piece of work with a start and an outcome. `con.Start` opens
it and `Phase` names the step it is on. `End` closes it with one of the
three outcomes: `Done` when the error is nil, `Cancelled` when the context
was cancelled, `Failed` otherwise.

<!-- code: examples/hello/main.go#greetTask -->
```go
func greetTask(ctx context.Context, name string) error {
    t := con.Start("Greeting " + name)
    t.Phase("thinking")
    o := console.Outcomes{Done: "Greeted " + name, Failed: "Failed to greet " + name, Cancelled: "Cancelled greeting " + name}
    var err error
    select {
    case <-ctx.Done():
        err = ctx.Err()
    case <-time.After(300 * time.Millisecond):
    }
    return t.End(ctx, err, o)
}
```

On a terminal the task is a live line with its phase. In a log,
`hello greet world --color never` prints the lines that stay:

<!-- capture: hello-greet.txt -->
```text
INFO Hello, world
OK Greeted world
```

## Return an error with a hint

A command fails by returning an error. `errNoSuchPerson` is
`errors.New("no such person")`. clix prints the error as the final `ERROR`
line, capitalises the first letter, and exits 1.

<!-- code: examples/hello/main.go#greetError -->
```go
func greetError(name string) error {
    return fmt.Errorf("%w: %s", errNoSuchPerson, name)
}
```

The `Hint` function in the options maps an error to one line of advice
printed under it.

<!-- code: examples/hello/main.go#hint -->
```go
func hint(err error) string {
    if errors.Is(err, errNoSuchPerson) {
        return "Try `hello greet world`"
    }
    return ""
}
```

`hello greet nobody --color never` prints this, with the exit status
shown on the last line:

<!-- capture: hello-error.txt -->
```text
ERROR No such person: nobody
  hint: Try `hello greet world`
---exit 1---
```

## Hand the commands to clix

`clix.Run` runs the command and exits with the right code. `Setup` runs
once the console exists and before the command, so it keeps the console.

<!-- code: examples/hello/main.go#main -->
```go
func main() {
    clix.Run(newRoot(), clix.Options{
        App:       "hello",
        Version:   "0.1.0",
        EnvPrefix: "HELLO",
        Hint:      hint,
        Setup: func(ctx context.Context, c *console.Console) (context.Context, error) {
            con = c
            return ctx, nil
        },
    })
}
```

The program now has four global flags: `--color`, `--narration`, `-q` or
`--quiet`, and `--verbose`. With `EnvPrefix: "HELLO"`, the variable
`HELLO_NARRATION` sets the default for `--narration`; the flag wins.

## Read the same run from a script

`hello greet world --narration json` prints the same run as one JSON
object per line, with the phase that a log leaves out:

<!-- capture: hello-greet.jsonl -->
```text
{"seq":1,"time":"2026-10-20T02:55:15.572500Z","event":"begin","app":"hello","version":"0.1.0","schema":1}
{"seq":2,"time":"2026-10-20T02:55:15.572730Z","event":"line","level":"info","text":"Hello, world"}
{"seq":3,"time":"2026-10-20T02:55:15.572741Z","event":"task","task":1,"state":"started","label":"Greeting world"}
{"seq":4,"time":"2026-10-20T02:55:15.572754Z","event":"task","task":1,"state":"phase","phase":"thinking"}
{"seq":5,"time":"2026-10-20T02:55:15.873788Z","event":"task","task":1,"state":"done","outcome":"Greeted world","elapsed_ms":301}
{"seq":6,"time":"2026-10-20T02:55:15.874018Z","event":"exit","code":0}
```

## Test what the user sees

`consoletest.Transcript` runs a real console with a fixed clock and
returns what it wrote to stderr. The test skips `clix.Run` and `Setup`,
so it sets `con` itself. It runs `hello greet world` and compares the
exact text. The full file with its imports, among them
`github.com/vi-dev/clix/console/consoletest`, is
[examples/hello/main_test.go](../examples/hello/main_test.go).

<!-- code: examples/hello/main_test.go#TestGreetTranscript -->
```go
func TestGreetTranscript(t *testing.T) {
    got := consoletest.Transcript(t, console.Options{Color: console.ColorNever}, func(c *console.Console, tick func()) {
        con = c
        root := newRoot()
        root.SetArgs([]string{"greet", "world"})
        if err := root.ExecuteContext(t.Context()); err != nil {
            t.Fatal(err)
        }
    })
    if want := "INFO Hello, world\nOK Greeted world\n"; got != want {
        t.Errorf("transcript:\n got %q\nwant %q", got, want)
    }
}
```

## Where to go next

- [The guide](guide/): [narration](guide/narration.md), [tasks](guide/tasks.md),
  [data](guide/data.md), [commands](guide/commands.md),
  [testing](guide/testing.md), [signals and exit](guide/signals-and-exit.md).
- [The reference](reference/): [flags and environment](reference/flags-and-environment.md),
  [JSON events](reference/json-events.md), [terminal](reference/terminal.md),
  [exit codes](reference/exit-codes.md).
- [Conventions](conventions.md): the house style for what a command prints.
