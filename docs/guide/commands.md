# Commands

This page is for a developer adding a command and deciding what it
prints. Afterwards you can give each command a shape that users and
scripts already know: a query, a change, a batch, a dry run, a question
before removing, or a child process run as is.

Each shape fixes what goes to stdout, what goes to stderr and how the run
ends. They are the defaults clix programs ship with, not rules; the
reasons behind the wording and the choices are in
[conventions](../conventions.md). The examples come from `app`, the demo
program in [examples/app/main.go](../../examples/app/main.go). In a
capture, stderr comes first, stdout follows a `---stdout---` line, and a
last `---exit N---` line shows a status other than 0.

## Show a result

A query changes nothing. Its result is the data on stdout, as `status`
and `search` show in [data](data.md#print-a-table), with at most one line
on stderr saying what to do next. Finding nothing is not a failure: the
`WARN No packages match "x"` run on that page exits 0.

## Change one thing

A one-shot change runs one task per unit of work, and each task ends with
one line in the past tense with its time. It writes nothing to stdout
unless the result is data, such as a script for the shell. `install` in
[tasks](tasks.md#start-a-task-and-end-it) is one, and its run prints a
single `OK Installed jq 1.8.3 (2s)` line.

When the thing is already done, the run ends with a Success line, such as
`OK jq up to date (1.8.2)`, and exits 0.

## Run one verb over many items

A batch runs the same unit of work for each item, keeps going past
failures and ends with one summary line.

<!-- code: examples/app/main.go#checkPackages -->
```go
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
```

Named items each get a line, up to date or not. A failed item gets a
`WARN` line with its cause and a failed task line, and the run goes on.
`app check tool broken jq --color never` prints:

<!-- capture: check-explicit.txt -->
```text
OK Checked tool (1 step)
WARN broken: test step 1 ("false"): exit status 1
ERROR Failed to check broken
OK jq up to date (1.8.2)
OK Checked 1 package, 1 failed, 1 up to date
  hint: Retry with `app check broken`
---exit 1---
```

With no names the command sweeps every item. An item with nothing to do
gets no line of its own, only a Debug line, and is counted in the summary;
`checkPackage` in [tasks](tasks.md#close-a-task-yourself) does this.
`app check --verbose --color never` prints:

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

The summary lists the outcome first, then failures, then the neutral
counts, and leaves out a count of zero; with nothing processed it reads
`Nothing to check`. A hint for the whole run comes once, after it.
Returning `&clix.ExitError{Code: 1}` exits 1 with no final `ERROR` line,
because the summary already said what failed.

## Stop at the first failure

Some batches cannot go on after a failure. `publishCatalog`, shown in
[data](data.md#keep-stdout-for-the-result), compares first, then pushes,
and returns the first push that fails, which ends the run.
`app publish registry.example/cat --color never` prints:

<!-- capture: publish.txt -->
```text
OK Compared with registry.example/cat: 1 to push, 41 unchanged
OK Pushed jq 1.8.3
OK Published registry.example/cat: 1 pushed, 41 unchanged
```

## Show the plan with --dry-run

A dry run may read but never writes, downloads or pushes. The plan is
data: a table on stdout of what would change, and nothing when nothing
would. The narration says `Would …` rather than starting with
`Dry run:`. `app publish registry.example/cat --dry-run --color never`
prints:

<!-- capture: publish-dry-run.txt -->
```text
OK Compared with registry.example/cat: 1 to push, 41 unchanged
OK Would publish registry.example/cat: 1 to push, 41 unchanged
---stdout---
PACKAGE  VERSION  STATUS
jq       1.8.3    updated
```

With `--output json`, the same rows go out as one JSON document, see
[data](data.md#offer-json-with---output).

## Ask before removing

A command that deletes or overwrites asks first, and offers `--yes` to
skip the question and `--dry-run` to stop after the plan. The question
and its answer come from `confirm`, shown in
[narration](narration.md#ask-a-question).

<!-- code: examples/app/main.go#cleanVersions -->
```go
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
```

`printf n | app clean --color never` declines:

<!-- capture: clean-prompt.txt -->
```text

? Remove these package versions? [y/N]: n
OK Nothing to remove
---stdout---
PACKAGE  VERSION  SIZE
jq       1.7.1    1 MB
zlib     1.3      512 KB
```

`app clean --yes --color never` removes without asking:

<!-- capture: clean-yes.txt -->
```text
OK Removed 2 package versions
---stdout---
PACKAGE  VERSION  SIZE
jq       1.7.1    1 MB
zlib     1.3      512 KB
```

When input ends before an answer, the empty answer still ends the line,
so the next line does not start after the question. `app clean --color
never < /dev/null` prints:

<!-- capture: clean-eof.txt -->
```text

? Remove these package versions? [y/N]: 
OK Nothing to remove
---stdout---
PACKAGE  VERSION  SIZE
jq       1.7.1    1 MB
zlib     1.3      512 KB
```

## Hand the terminal to a child

A command that runs another program for the user connects the child to
its own stdin, stdout and stderr, and clix adds no line of its own.
`ExitError` passes the child's exit status on, see
[signals and exit](signals-and-exit.md#choose-the-exit-code).

<!-- code: examples/app/main.go#runPassthrough -->
```go
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
```

`app run --color never -- sh -c 'echo out; echo err >&2; exit 3'` prints:

<!-- capture: run-passthrough.txt -->
```text
err
---stdout---
out
---exit 3---
```

## Pass a child's output through a task

When the child is one step of the command's own work, its output goes
through the task's writers instead, as `runStep` does in
[tasks](tasks.md#pass-a-childs-output-through-the-task). On a terminal it
appears above the live lines, never inside one. In a log the task first
announces itself with an `INFO` line, so the child's lines have a
subject. `app build broken@1.0 --color never` prints:

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

## Where to go next

- [Conventions](../conventions.md): why the summaries, hints and prompts
  read the way they do.
- [Testing](testing.md): check what each shape prints.
- [Signals and exit](signals-and-exit.md): exit codes and Ctrl-C.
