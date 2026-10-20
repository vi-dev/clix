# Narration

This page is for a developer whose commands talk to the user. Afterwards
you can write a line at each level, fail with an error and a hint, know
what `--quiet` and `--verbose` keep, and ask a question.

Narration is everything a command tells the person running it: progress,
outcomes, warnings, errors, hints and prompts. It goes to stderr. A result
that a script reads goes to stdout, see [data](data.md). The examples come
from `app`, the demo program in
[examples/app/main.go](../../examples/app/main.go). In a capture, stderr
comes first, stdout follows a `---stdout---` line, and a last
`---exit N---` line shows a status other than 0.

## Write a line

`clix.ConsoleOf(cmd)` returns the console of the running command. Each
level has a method: `Info`, `Success`, `Warn`, `Debug` and `Hint`.

<!-- code: examples/app/main.go#narrateUpdate -->
```go
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
```

The first line is the outcome of the task that `updateCatalog` runs, see
[tasks](tasks.md). `app update --color never` prints:

<!-- capture: update.txt -->
```text
OK Updated catalog main
WARN Catalog extra is 9 days old — run `app update extra`
INFO Release notes: https://registry.example/notes
```

Each method has an `f` twin. `Info` joins its arguments with spaces, like
`fmt.Sprintln`; `Infof` formats them, like `fmt.Sprintf`. Fixed text and
text that is not a format, such as a name read from a file, go to `Info`,
so a `%` in it prints as is. `Warnf` above fills in values.

## Know what each level looks like

| Method                | Plain               | In colour                     | Under `--quiet`                |
|-----------------------|---------------------|-------------------------------|--------------------------------|
| `Info`, `Infof`       | `INFO msg`          | cyan `•`, then the text       | hidden                         |
| `Success`, `Successf` | `OK msg`            | green `✓`, then the text      | hidden                         |
| `Warn`, `Warnf`       | `WARN msg`          | yellow `!`, then the text     | shown                          |
| `Error`               | `ERROR msg`         | the whole line red, `✗ msg`   | shown                          |
| `Hint`, `Hintf`       | `  hint: msg`       | two spaces, then dim `→ msg`  | hidden, except under an error  |
| `Debug`, `Debugf`     | `DEBUG msg`         | the whole line dim, `· msg`   | hidden, as without `--verbose` |
| `Prompt`, `Promptf`   | `? msg`, no newline | cyan `?`, then the text       | shown                          |
| `Answer`              | `msg`               | `msg`                         | shown                          |

Colour is on when stderr is a terminal, and `--color always` turns it on
anywhere. The flags and variables that decide it are in
[flags and environment](../reference/flags-and-environment.md).
`app update --verbose --color always` prints this, with each escape byte
shown as `\x1b`:

<!-- capture: update-color.txt -->
```text
\x1b[32m✓\x1b[0m Updated catalog main
\x1b[2m· Skipped jq 1.8.3 (unchanged)\x1b[0m
\x1b[33m!\x1b[0m Catalog extra is 9 days old — run `app update extra`
\x1b[36m•\x1b[0m Release notes: https://registry.example/notes
```

The colour codes and the live lines that tasks draw on a terminal are in
[terminal](../reference/terminal.md). Which level suits which message is
in [conventions](../conventions.md).

## Show more with --verbose, less with --quiet

`--verbose` adds the Debug lines:

<!-- capture: update-verbose.txt -->
```text
OK Updated catalog main
DEBUG Skipped jq 1.8.3 (unchanged)
WARN Catalog extra is 9 days old — run `app update extra`
INFO Release notes: https://registry.example/notes
```

`--quiet` keeps warnings, errors and prompts, and drops the rest:

<!-- capture: update-quiet.txt -->
```text
WARN Catalog extra is 9 days old — run `app update extra`
```

A failed task and the final error stay too, so a quiet run that fails
still says why: `app install zlib@9.9 --quiet` prints the same two `ERROR`
lines as the run in [tasks](tasks.md#fail-a-task).

## Fail with an error

A command fails by returning an error. clix prints it as the final
`ERROR` line and exits 1, so the command does not print it too. Only the
first letter is capitalised; the rest of the wrapped error stays as
written, outside in, such as `Download zlib 9.9: 404 Not Found`.

<!-- code: examples/app/main.go#installPackage -->
```go
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
```

A message of several lines keeps its line breaks, and each further line is
indented under the first. Here `install` returns
`fmt.Errorf("%w\nwant sha256:…\ngot  sha256:…", errChecksum)`, and clix
prints it after the task's own failure line:

<!-- capture: install-corrupt.txt -->
```text
ERROR Failed to install corrupt 1.0
ERROR Checksum mismatch
      want sha256:9f86d081884c7d65
      got  sha256:60303ae22b998861
  hint: Run `app install corrupt@1.0 --verbose`
---exit 1---
```

`Error(err, hint)` prints the same lines from the console. clix calls it
for the returned error, so a command seldom needs it.

## Attach a hint

`Options.Hint` maps the final error to one line of advice, printed under
it. The demo's `hint` takes the advice from a wrapping error, as in the
checksum case above, or from a known error such as `errNotFound`.

<!-- code: examples/app/main.go#hint -->
```go
func hint(err error) string {
    if h, ok := errors.AsType[*hintedError](err); ok {
        return h.hint
    }
    if errors.Is(err, errNotFound) {
        return "Check the package name or run `app update`"
    }
    return ""
}
```

`app install nosuch --color never` prints:

<!-- capture: install-nosuch.txt -->
```text
ERROR Package nosuch not found
  hint: Check the package name or run `app update`
---exit 1---
```

A hint under any other line is `Hint` or `Hintf` and is hidden under
`--quiet`. What a hint says is in [conventions](../conventions.md).

## Ask a question

`Prompt` writes the question with no newline and shows under `--quiet`,
because a silent wait looks like a hang. `Answer` echoes what was read and
ends the line, so question and answer share one line in every log.
`BlankErr` writes one blank line on stderr before the question, and none
under `--quiet`.

<!-- code: examples/app/main.go#confirm -->
```go
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
```

`printf n | app clean --color never` prints the plan table on stdout and
the question on stderr:

<!-- capture: clean-prompt.txt -->
```text

? Remove these package versions? [y/N]: n
OK Nothing to remove
---stdout---
PACKAGE  VERSION  SIZE
jq       1.7.1    1 MB
zlib     1.3      512 KB
```

The full shape of a command that asks, with `--yes` and `--dry-run`, is in
[commands](commands.md).

## Where to go next

- [Tasks](tasks.md): work that takes time, with phases, progress and an
  outcome.
- [Data](data.md): what goes to stdout.
- [Conventions](../conventions.md): levels, wording and hints.
- [Terminal](../reference/terminal.md): colour codes and the live lines.
