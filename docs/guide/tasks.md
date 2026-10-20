# Tasks

This page is for a developer whose commands do work that takes time.
Afterwards you can run that work as a task with phases and progress, end
it as done, failed or cancelled, and pass a child process's output
through it.

A task is one piece of work with one subject: an install, a download, a
build. Anything that may take over a second is a task, so a terminal shows
it running and a log keeps its outcome. The examples come from `app`, the
demo program in [examples/app/main.go](../../examples/app/main.go).

## Start a task and end it

`Start` opens a task with its label. `End` closes it from an error with one
of three `Outcomes`: `Done` when the error is nil, `Cancelled` when the
context was cancelled, `Failed` otherwise. `console.FromContext(ctx)`
returns the [`Narrator`](#hand-narration-to-other-code) that clix puts in
the command's context, so code below the command needs no console
argument.

<!-- code: examples/app/main.go#install -->
```go
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
```

`app install jq@1.8.3 --color never` prints:

<!-- capture: install-ok.txt -->
```text
OK Installed jq 1.8.3 (2s)
```

A Done line adds its time: nothing under a second, `(3s)` under a minute,
`(1m05s)` after that. On a terminal the running task is a live line, such
as `Installing jq 1.8.3  downloading 49%`; [terminal](../reference/terminal.md)
describes how those lines are drawn.

## Follow a task from a script

With `--narration json` the same run shows each phase and a progress
sample, at most one a second:

<!-- capture: install-ok.jsonl -->
```text
{"seq":1,"time":"2026-10-20T04:34:56.944339Z","event":"begin","app":"app","version":"0.1.0","schema":1}
{"seq":2,"time":"2026-10-20T04:34:56.944659Z","event":"task","task":1,"state":"started","label":"Installing jq 1.8.3"}
{"seq":3,"time":"2026-10-20T04:34:56.944678Z","event":"task","task":1,"state":"phase","phase":"downloading"}
{"seq":4,"time":"2026-10-20T04:34:57.946841Z","event":"task","task":1,"state":"progress","phase":"downloading","done":3137536,"total":4194304,"unit":"bytes"}
{"seq":5,"time":"2026-10-20T04:34:58.451186Z","event":"task","task":1,"state":"phase","phase":"extracting"}
{"seq":6,"time":"2026-10-20T04:34:58.552274Z","event":"task","task":1,"state":"done","outcome":"Installed jq 1.8.3","elapsed_ms":1607}
{"seq":7,"time":"2026-10-20T04:34:58.552490Z","event":"exit","code":0}
```

Each event and its fields are in [JSON events](../reference/json-events.md).

## Report progress

`ProgressReader(t, r, total)` counts bytes read and `ProgressWriter(t,
total)` counts bytes written, as in `install`. A total of `-1` means it is
unknown, and the line then shows the bytes so far, such as `12 MB`. Bytes
show as `512 B`, `12 KB`, `3 MB`, with no decimals. For a count of items, call
`Progress` with `console.Items`, as `buildPackage` below does in its
`compiling` phase:

<!-- code: examples/app/main.go#compileSlowly -->
```go
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
```

On a terminal the line reads `Building slow 1.0  compiling 1/2`, and items
with no total show the bare count, such as `3`. Nothing shows before a
phase is set. A new phase clears the progress of the one before; setting
the same phase again keeps it. A phase with no progress for ten seconds
shows its age, as in `extracting (12s)`. `app build slow@1.0 --narration
json` prints the sample:

<!-- capture: build-slow.jsonl -->
```text
{"seq":1,"time":"2026-10-20T04:34:06.721486Z","event":"begin","app":"app","version":"0.1.0","schema":1}
{"seq":2,"time":"2026-10-20T04:34:06.721594Z","event":"task","task":1,"state":"started","label":"Building slow 1.0"}
{"seq":3,"time":"2026-10-20T04:34:06.721612Z","event":"task","task":1,"state":"phase","phase":"compiling"}
{"seq":4,"time":"2026-10-20T04:34:07.723776Z","event":"task","task":1,"state":"progress","phase":"compiling","done":1,"total":2,"unit":"items"}
{"seq":5,"time":"2026-10-20T04:34:08.124054Z","event":"task","task":1,"state":"done","outcome":"Built slow 1.0","elapsed_ms":1402}
{"seq":6,"time":"2026-10-20T04:34:08.124190Z","event":"exit","code":0}
```

## Fail a task

When `End` gets an error that is not a cancellation, the task prints its
`Failed` outcome as an `ERROR` line, with no time, and returns the error
for clix to print after it. Both lines show under `--quiet` too.
`app install zlib@9.9 --color never` prints:

<!-- capture: install-fail.txt -->
```text
ERROR Failed to install zlib 9.9
ERROR Download zlib 9.9: 404 Not Found
---exit 1---
```

## Cancel a task

When the context is cancelled, `End` prints the `Cancelled` outcome as an
`ERROR` line instead. After Ctrl-C clix adds no final error and exits 130;
[signals and exit](signals-and-exit.md) shows the whole run and the other
signals.

## Close a task yourself

`Done`, `Fail` and `Discard` close a task with a text you choose.
`Discard` prints nothing, for an item that turned out to need no work.
Lines written through the task, such as `t.Warnf`, belong to it: in JSON
they carry its number, and once it is closed they are dropped.

<!-- code: examples/app/main.go#checkPackage -->
```go
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
```

`app check --verbose --color never` checks every package. jq needs no
work, so its task is discarded and a Debug line counts it:

<!-- capture: check-sweep-verbose.txt -->
```text
WARN broken: test step 1 ("false"): exit status 1
ERROR Failed to check broken
DEBUG Skipped jq (up to date)
OK Checked tool (1 step)
OK Checked 1 package, 1 failed, 1 up to date
  hint: Retry with `app check broken`
---exit 1---
```

The summary and its hint come from `checkPackages`, a batch command shown
in [commands](commands.md).

## Run a function as a task

`console.RunTask` starts a task, sets its first phase, passes the
function a progress callback, and ends the task from the error it returns.

<!-- code: examples/app/main.go#updateCatalog -->
```go
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
```

Its outcome, `OK Updated catalog main`, is the first line of `app update`
in [narration](narration.md#write-a-line).

## Pass a child's output through the task

A task's `Out` and `ErrOut` are writers for stdout and stderr. Output
written through them never lands inside a live line, and in a log the task
first announces itself with an `INFO` line, so the output has a subject.

<!-- code: examples/app/main.go#buildPackage -->
```go
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
```

`runStep` runs one build step with both of its streams on the task's
`ErrOut`:

<!-- code: examples/app/main.go#runStep -->
```go
func runStep(ctx context.Context, t console.Task, script string) error {
    step := exec.CommandContext(ctx, "sh", "-c", script)
    step.Stdout = t.ErrOut()
    step.Stderr = t.ErrOut()
    return step.Run()
}
```

`app build broken@1.0 --color never` prints:

<!-- capture: build-broken.txt -->
```text
INFO Building broken 1.0
checking for cc... no
ERROR Failed to build broken 1.0
ERROR Configure failed: exit status 1
      see config.log
  hint: Run `app build broken@1.0 --verbose`
---exit 1---
```

For output that belongs to no task, the console has its own `Out` and
`ErrOut`.

## Run tasks one after another

Each unit of work is its own task with its own outcome, and one summary
line closes the batch.

<!-- code: examples/app/main.go#syncPackages -->
```go
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
```

`app sync --color never` prints:

<!-- capture: sync.txt -->
```text
OK Installed jq 1.8.3 (2s)
OK Installed zlib 1.3.1 (2s)
OK Synced 2 packages
```

On a terminal, every running task is one line of the live block, in the
order they started; [terminal](../reference/terminal.md) describes it.

## Hand narration to other code

`console.Narrator` is the set of line methods, `Start`, `Out` and `ErrOut`
that a `*Console` has. Code below a command can take a `Narrator` instead
of the console, and `console.Prefixed(n, "jq: ")` returns one that starts
every line and task label with the prefix.

## Where to go next

- [Narration](narration.md): the lines around tasks.
- [Commands](commands.md): batch commands, dry runs and child processes.
- [Testing](testing.md): assert on phases, progress and outcomes.
- [JSON events](../reference/json-events.md) and
  [terminal](../reference/terminal.md): what a task produces in each mode.
