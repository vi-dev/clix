# Conventions

This page is for a developer choosing what a command prints. Afterwards
you can pick the level of a line, word it, write its hint, sum up a batch
and shape the help text the way other clix programs do.

These are the defaults clix programs ship with. A program may deviate; it
stays consistent with itself. Each entry gives its reason, then a run of
`app`, the demo program in [examples/app/main.go](../examples/app/main.go),
with `--color never`. In a capture, stderr comes first, stdout follows a
`---stdout---` line, and a last `---exit N---` line shows a status other
than 0. A `Bad:` block is made up to show the contrast.

## Keep the two planes apart

**Data goes to stdout, narration to stderr.** A script reads stdout and a
person reads stderr, so either can be redirected without losing the other.
Good, `app status`:

<!-- capture: status.txt -->
```text

WARN 1 package not installed — run `app sync`
---stdout---
PACKAGE  VERSION  CATALOG  INSTALLED
jq       1.8.3    main     yes
zlib     1.3.1    -        no
```

**Narration describes, data delivers.** A script that wants the answer
never has to parse a narration line for it. Good, `app info jq`:

<!-- capture: info.txt -->
```text
---stdout---
name:       jq
version:    1.8.3
homepage:   https://jqlang.org
catalog:    main
```

Bad: the same facts as `INFO jq 1.8.3 from catalog main` on stderr.

**Quiet means problems and data only**: warnings, errors, questions and
stdout. Someone who asks for quiet still needs to know why a run failed.
[Narration](guide/narration.md#show-more-with---verbose-less-with---quiet)
shows what each level does under `--quiet`.

## Choose the level

`app update --verbose` prints a line at four levels:

<!-- capture: update-verbose.txt -->
```text
OK Updated catalog main
DEBUG Skipped jq 1.8.3 (unchanged)
WARN Catalog extra is 9 days old — run `app update extra`
INFO Release notes: https://registry.example/notes
```

**`INFO` is for progress nobody counts**: plan lines, totals, pointers.
It hides under `--quiet`, so nothing on it is something the user has to
act on. Bad: an outcome such as `INFO Installed jq 1.8.3`.

**`WARN` is for a problem the command recovered from or counted**, never
the final error. It is the one line of this run left under `--quiet`, so
it is kept for what needs attention.

**`OK` is for an outcome the user asked for and for a batch summary.** A
reader scanning a log looks for these lines to see what happened. Good,
`app sync`:

<!-- capture: sync.txt -->
```text
OK Installed jq 1.8.3 (2s)
OK Installed zlib 1.3.1 (2s)
OK Synced 2 packages
```

**`ERROR` is the final line of a failed command, or a task's failure
line**, and nothing else. Whoever sees `ERROR` knows the run did not do
what was asked. Good, `app install zlib@9.9`:

<!-- capture: install-fail.txt -->
```text
ERROR Failed to install zlib 9.9
ERROR Download zlib 9.9: 404 Not Found
---exit 1---
```

**`DEBUG` is for per-item no-ops in a sweep and for maintainer detail**,
and a discarded task prints nothing else. The summary already counts the
no-ops, and `--verbose` brings them back for whoever needs them. Bad, one
`OK` line per no-op in a sweep of hundreds:

```text
$ app check
OK Skipped jq (up to date)
OK Skipped zlib (up to date)
```

A hint follows the line it explains, see [Write a hint](#write-a-hint).
Questions and answers are in [narration](guide/narration.md#ask-a-question);
reading a single key on a terminal is the program's choice, and `app`
reads a whole line.

## Name the subject

**Every line names its subject.** In a log of hundreds of lines,
`OK Installed` alone says nothing; `OK Installed jq 1.8.3 (2s)` in the
`app sync` run above says what.

**A task's label and its outcome are one sentence in different tenses**,
so the reader matches the line that ran to the line that ended it. Done
adds its time, so slow work stands out; Failed has none.

| Situation | Form                               | Example |
|-----------|------------------------------------|------------------------------------|
| Running   | present participle + subject       | `Installing jq 1.8.3` |
| Done      | `OK` past participle + subject     | `OK Installed jq 1.8.3 (2s)` |
| Failed    | `ERROR Failed to <verb> <subject>` | `ERROR Failed to install zlib 9.9` |
| Cancelled | `ERROR Cancelled <subject>`        | `ERROR Cancelled jq 1.8.3` |
| Dry run   | `Would <verb> …`                   | `OK Would publish registry.example/cat: 1 to push, 41 unchanged` |

**The failure label names its verb.** "Failed" alone does not say which
part of a run broke: `ERROR Failed to install zlib 9.9` above, not
`ERROR Update failed`.

**One task per unit of work**, so the log has one outcome line per thing
the user asked for. A build walks its phases under one task; a follow-up
with its own subject, such as a push, is its own task. Nested
tasks appear only when a phase runs other units of work. Good,
`app publish registry.example/cat`:

<!-- capture: publish.txt -->
```text
OK Compared with registry.example/cat: 1 to push, 41 unchanged
OK Pushed jq 1.8.3
OK Published registry.example/cat: 1 pushed, 41 unchanged
```

Bad, phases run as tasks:

```text
$ app install jq@1.8.3
OK Downloaded jq 1.8.3
OK Extracted jq 1.8.3
```

**Phases are lowercase gerunds**, with an optional object, or a step
counter: `downloading`, `pulling index`, `step 2/5`. They read as the end
of the running label, `Installing jq 1.8.3  downloading`, as the runs in
[tasks](guide/tasks.md#report-progress) show.

**A problem with one item reads `<subject>: <detail>`**, with version and
platform when they matter, such as `jq 1.8.3 linux/amd64: checksum
mismatch`. **An item already done reads `<subject> up to date
(<version>)`.** Both put the subject first, where the eye starts. Good,
`app check tool broken jq`:

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

## Write the sentence

**Capital first letter, no final period.** Lines are labels in a log, and
one shape everywhere makes them easy to scan: `OK Greeted world`, not
`OK greeted world.`, from `hello greet world` in
[getting started](getting-started.md):

<!-- capture: hello-greet.txt -->
```text
INFO Hello, world
OK Greeted world
```

**One line per fact.** A line can then be grepped, counted and given its
own level, as in the `app update` run above. Bad, a warning hidden in an
outcome:

```text
$ app update
OK Updated catalog main, catalog extra is 9 days old
```

**Commands to run go in backticks, joined to the problem by an em dash.**
The reader can copy the command, and the dash marks where the problem
ends and the next step begins. Good: the `WARN` line of `app status` at
the top of this page. Bad: the same line as
`WARN 1 package not installed, run app sync`.

**Secondary detail goes in parentheses at the end, and a version change
uses a spaced arrow**: `OK Checked tool (1 step)`, `Updated jq 1.8.2 →
1.8.3`. The line still starts with what happened, and the arrow reads as
"from, to" at a glance.

**One-shot successes are past-participle sentences, and counted nouns
agree with the count**, as in `OK Removed 2 package versions` from the
`app clean --yes` run in [commands](guide/commands.md#ask-before-removing).
A form like `2 package version(s)` reads as if the program did not know.

**One noun per thing**, in help and narration alike. A second word for
the same thing makes the reader wonder whether it is a second thing:
`app` says "package" everywhere, never "tool" as well.

## Write a hint

**A hint is one imperative line that names the exact command.** The
reader can act on it at once, without reading on. Good, `hello greet
nobody`:

<!-- capture: hello-error.txt -->
```text
ERROR No such person: nobody
  hint: Try `hello greet world`
---exit 1---
```

Bad, a second sentence of explanation:

```text
$ app install nosuch
ERROR Package nosuch not found
  hint: The package may not exist in any of the configured catalogs
```

A hint for a whole batch comes once, after the summary, as in the
`app check` run above. See also
[narration](guide/narration.md#fail-with-an-error) for the final error and
[signals and exit](guide/signals-and-exit.md#choose-the-exit-code) for its
exit code.

## Sum up a batch

**A batch ends with one summary line: the outcome first, then failures,
then neutral counts.** Groups are joined by `, ` and a count of zero is
left out, so the line says what got done before anything else. The
`app check` run above ends with `OK Checked 1 package, 1 failed, 1 up to
date`. Bad:

```text
$ app check tool broken jq
OK 1 failed, 1 up to date, 1 checked, 0 skipped
```

The summary is `OK`, or `WARN` when a program wants failures to stand
out. Where every count matters, a fixed shape is fine, such as
`1 pushed, 41 unchanged` in the `app publish` run above.

**Nothing to do is `Nothing to <verb>` or `All packages up to date`, at
`OK`, and exits 0**, keeping any parenthetical, as in `Nothing to check
(1 without tests)`. Having no work is a success, not a problem, so it is
never `WARN Nothing to check`.

See also [commands](guide/commands.md#run-one-verb-over-many-items) for
named items, sweeps and the exit status of a batch.

## Shape the help text

Command descriptions are imperative fragments, capitalised, with no
period, such as `Install a package`. Flag descriptions are lowercase
fragments with no period, the choices spelled out and the default at the
end. Both read as a list the eye runs down. Every visible command has a
long description of two to five full sentences in present tense and
plain text. It also has two to four example lines indented two spaces
with `# comments` in one column, so the reader learns what a command
touches before running it. Good, `app install --help`:

<!-- capture: help-install.txt -->
```text
---stdout---
Install resolves a package in the configured catalogs, downloads it,
and adds it to the project. It writes the package under the app home
and records the exact version.

Usage:
  app install <name>@<version> [flags]

Examples:
  app install jq               # newest version in the first catalog
  app install jq@1.8.3         # an exact version

Flags:
  -h, --help   help for install

Global Flags:
      --color string       colorize output: auto, always, or never (default "auto")
      --narration string   narration format: auto, plain, or json (default "auto")
  -q, --quiet              suppress narration output
      --verbose            show debug output
```

**Once there are more than a handful of commands, group them by
purpose** in the root help, so the reader finds a command by what it is
for; the demo keeps a flat list.

**Machine-readable output is `--output text|json`** on every command
that offers it, never a separate `--json`, so a script learns one flag;
see [data](guide/data.md#offer-json-with---output). A `--dry-run` flag
gives the plan described in
[commands](guide/commands.md#show-the-plan-with---dry-run).
