# Terminal

This page is for anyone who needs the exact bytes a clix program writes
to stderr: a person testing raw output, a terminal recorder, a program
that narrates text it did not write. It covers the live block, plain
output, width and colour. A program can rely on all of it: a change to
this page is a breaking change. The JSON stream is in
[JSON events](json-events.md).

The examples come from `app`, the demo program in
[examples/app/main.go](../../examples/app/main.go), with each escape
byte shown as `\x1b`.

## Live and plain output

Narration is live when `--narration` is `auto` and stderr is a terminal;
otherwise it is plain (see
[flags and environment](flags-and-environment.md#--narration)). Live
output is plain lines plus the live block below them. `--color never`
and `NO_COLOR` keep the live block; `--quiet` hides it.

Plain output is a static transcript: lines in the order they happen,
never a cursor movement or an erase. With no running lines to show,
each task that has started and not yet been announced prints an info
line with its label just before the first bytes any task or the program
writes through clix's writers. The line is `INFO <label>` or, in colour,
a cyan `•` and the label.

## The live block

The live block has one line per running task, in start order:

- `<frame> <label>` before the task has a phase;
- `<frame> <label>  <phase>` with a phase, and `<phase> (12s)` once the
  phase has run ten seconds without progress (setting the phase again
  restarts the count);
- `<frame> <label>  <phase> <progress>` with progress.

Each line starts with `\x1b[2K` (erase the line) and ends with `\n`. A
redraw first moves the cursor up over the old block with `\x1b[<n>A`; a
clear moves up and erases to the end of the screen with `\x1b[<n>A\x1b[J`.
Every redraw or clear is one write to stderr, so the block is never left
half drawn. The cursor is never hidden.

| When                                    | What happens |
|-----------------------------------------|--------------|
| a task starts                           | the block is redrawn with the new line |
| every 100 ms while a task runs          | the block is redrawn and the frame moves on; new phases and progress show here |
| a narration line or a blank line        | the block is cleared, the line printed, the block redrawn |
| a task ends                             | the block is cleared, the outcome line printed, the block redrawn without the task |
| a task or the program writes to stderr  | the block is cleared and the bytes follow; the block returns on the next tick |
| a task or the program writes to stdout  | the block is cleared on stderr, even when stdout is a pipe, then the bytes go to stdout; the block returns on the next tick |
| data goes to stdout                     | the block is cleared first only when stdout and stderr are both terminals; it returns on the next tick |
| the run exits                           | a block still on screen is removed; printed lines stay |

The frames are the braille set `⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏` when colour is on and
`|/-\` otherwise. Every line of a redraw shares one frame. A finished
task leaves the block and its outcome line prints above it, with `✓` or
`✗` in place of the frame when colour is on.

`app build slow@1.0 --color always` on an 80-column terminal, one line
per write:

<!-- capture: unchecked/live-80.txt -->
```text
\x1b[2K\x1b[36m⠋\x1b[0m Building slow 1.0
\x1b[1A\x1b[2K\x1b[36m⠙\x1b[0m Building slow 1.0  \x1b[2mcompiling 0/2\x1b[0m
\x1b[1A\x1b[2K\x1b[36m⠹\x1b[0m Building slow 1.0  \x1b[2mcompiling 0/2\x1b[0m
\x1b[1A\x1b[2K\x1b[36m⠸\x1b[0m Building slow 1.0  \x1b[2mcompiling 0/2\x1b[0m
\x1b[1A\x1b[2K\x1b[36m⠼\x1b[0m Building slow 1.0  \x1b[2mcompiling 0/2\x1b[0m
\x1b[1A\x1b[2K\x1b[36m⠴\x1b[0m Building slow 1.0  \x1b[2mcompiling 0/2\x1b[0m
\x1b[1A\x1b[2K\x1b[36m⠦\x1b[0m Building slow 1.0  \x1b[2mcompiling 0/2\x1b[0m
\x1b[1A\x1b[2K\x1b[36m⠧\x1b[0m Building slow 1.0  \x1b[2mcompiling 0/2\x1b[0m
\x1b[1A\x1b[2K\x1b[36m⠇\x1b[0m Building slow 1.0  \x1b[2mcompiling 1/2\x1b[0m
\x1b[1A\x1b[2K\x1b[36m⠏\x1b[0m Building slow 1.0  \x1b[2mcompiling 1/2\x1b[0m
\x1b[1A\x1b[2K\x1b[36m⠋\x1b[0m Building slow 1.0  \x1b[2mcompiling 1/2\x1b[0m
\x1b[1A\x1b[2K\x1b[36m⠙\x1b[0m Building slow 1.0  \x1b[2mcompiling 1/2\x1b[0m
\x1b[1A\x1b[2K\x1b[36m⠹\x1b[0m Building slow 1.0  \x1b[2mcompiling 1/2\x1b[0m
\x1b[1A\x1b[2K\x1b[36m⠸\x1b[0m Building slow 1.0  \x1b[2mcompiling 1/2\x1b[0m
\x1b[1A\x1b[2K\x1b[36m⠼\x1b[0m Building slow 1.0  \x1b[2mcompiling 1/2\x1b[0m
\x1b[1A\x1b[J\x1b[32m✓\x1b[0m Built slow 1.0 (1s)
```

## Width

The block fits the width of the terminal on stderr, or 80 columns when
the width is unknown. A program started through clix's usual entry
point reads it again after the window is resized; one driven through the
entry point for tests and embedding reads it once per run.

- **A long line shortens its label first**, ending it in `…` and keeping
  the frame, phase and progress whole.
- **When fewer than two columns are left for the label**, the label is
  `…` and the line is cut from the right: at 18 columns
  `\ Installing jq 1.8.3  downloading 74%` becomes `\ …  downloading 7`.
- **Width is counted in characters**, one column each. Wide East Asian
  characters and emoji take two columns and combining marks none, so such
  a line can end short or wrap, and a wrapped line spoils the next redraw.
- **Only live lines are cut.** Plain lines are printed whole.

The same run on a 24-column terminal:

<!-- capture: unchecked/live-24.txt -->
```text
\x1b[2K\x1b[36m⠋\x1b[0m Building slow 1.0
\x1b[1A\x1b[2K\x1b[36m⠙\x1b[0m Buildi…  \x1b[2mcompiling 0/2\x1b[0m
\x1b[1A\x1b[2K\x1b[36m⠹\x1b[0m Buildi…  \x1b[2mcompiling 0/2\x1b[0m
\x1b[1A\x1b[2K\x1b[36m⠸\x1b[0m Buildi…  \x1b[2mcompiling 0/2\x1b[0m
\x1b[1A\x1b[2K\x1b[36m⠼\x1b[0m Buildi…  \x1b[2mcompiling 0/2\x1b[0m
\x1b[1A\x1b[2K\x1b[36m⠴\x1b[0m Buildi…  \x1b[2mcompiling 0/2\x1b[0m
\x1b[1A\x1b[2K\x1b[36m⠦\x1b[0m Buildi…  \x1b[2mcompiling 0/2\x1b[0m
\x1b[1A\x1b[2K\x1b[36m⠧\x1b[0m Buildi…  \x1b[2mcompiling 1/2\x1b[0m
\x1b[1A\x1b[2K\x1b[36m⠇\x1b[0m Buildi…  \x1b[2mcompiling 1/2\x1b[0m
\x1b[1A\x1b[2K\x1b[36m⠏\x1b[0m Buildi…  \x1b[2mcompiling 1/2\x1b[0m
\x1b[1A\x1b[2K\x1b[36m⠋\x1b[0m Buildi…  \x1b[2mcompiling 1/2\x1b[0m
\x1b[1A\x1b[2K\x1b[36m⠙\x1b[0m Buildi…  \x1b[2mcompiling 1/2\x1b[0m
\x1b[1A\x1b[2K\x1b[36m⠹\x1b[0m Buildi…  \x1b[2mcompiling 1/2\x1b[0m
\x1b[1A\x1b[2K\x1b[36m⠸\x1b[0m Buildi…  \x1b[2mcompiling 1/2\x1b[0m
\x1b[1A\x1b[2K\x1b[36m⠼\x1b[0m Buildi…  \x1b[2mcompiling 1/2\x1b[0m
\x1b[1A\x1b[J\x1b[32m✓\x1b[0m Built slow 1.0 (1s)
```

## Colour

When colour is on is decided in
[flags and environment](flags-and-environment.md#which-setting-wins). The
glyphs follow the colour setting, not the terminal: `--color never` on a
terminal prints the plain words and draws the ASCII frames, and
`--color always` into a file prints `✓` and the other coloured glyphs.
Output to a file is plain, so it has no frames. Plain output under
`--color always` carries colour codes; plain means no cursor movement,
not no escape codes. stdout is never coloured.

| Line              | Plain             | In colour |
|-------------------|-------------------|-----------|
| info              | `INFO msg`        | cyan `•`, then the text |
| success, task done | `OK msg (2s)`    | green `✓`, then the text |
| warning           | `WARN msg`        | yellow `!`, then the text |
| error             | `ERROR msg`       | the whole line red, `✗ msg` |
| error, more lines | six spaces, then the line | the whole line red, two spaces, then the line |
| task failed       | `ERROR outcome`   | the whole line red, `✗ outcome` |
| hint              | `  hint: msg`     | two spaces, then dim `→ msg` |
| debug             | `DEBUG msg`       | the whole line dim, `· msg` |
| prompt            | `? msg`, no newline | cyan `?`, then the text, no newline |
| answer            | `msg`             | `msg` |
| live line         | `<frame> <label>  <phase>` | cyan frame, plain label, dim phase and progress |

The codes are `\x1b[32m` green, `\x1b[33m` yellow, `\x1b[31m` red,
`\x1b[36m` cyan, `\x1b[2m` dim, each closed by `\x1b[0m`.
`app update --verbose --color always` prints:

<!-- capture: update-color.txt -->
```text
\x1b[32m✓\x1b[0m Updated catalog main
\x1b[2m· Skipped jq 1.8.3 (unchanged)\x1b[0m
\x1b[33m!\x1b[0m Catalog extra is 9 days old — run `app update extra`
\x1b[36m•\x1b[0m Release notes: https://registry.example/notes
```

## Progress and durations

- **Items** show `3/12`, or `3` when the total is unknown.
- **Bytes** show `42%`, rounded down, or `12 MB` when the total is
  unknown: 1024-based, no decimals, `B`, `KB`, `MB`, `GB` and up.
- **A done outcome ends with its duration**: nothing under a second,
  ` (3s)` under a minute, ` (1m05s)` otherwise, to the nearest second.

## Text is printed as is

clix prints narrated text and the bytes a task writes as they are, with
control characters and escape sequences included, in plain and live
output. Strip control characters from untrusted strings, such as file
names or server messages, before narrating them. JSON
output escapes everything.

## Known limits

- **A last child line without a newline runs into the next narration
  line**, because clix writes child bytes as they come and adds none.
- **Wide and combining characters misalign the block**, as described in
  [width](#width).

## Where to go next

- [Tasks](../guide/tasks.md): what a task shows while it runs.
- [Narration](../guide/narration.md): the levels behind each line.
- [JSON events](json-events.md): the same run as events.
