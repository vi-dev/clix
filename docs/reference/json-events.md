# JSON events

This page is for anyone who writes a program that reads a clix program's
narration: a CI step, an editor, an agent. It defines the stream that
`--narration json` writes to stderr. A consumer can rely on all of it: a
change to this page is a breaking change, and renaming or removing a
field or a value also raises `schema`.

The examples come from `app`, the demo program in
[examples/app/main.go](../../examples/app/main.go).

## The stream

With `--narration json`, or `<PREFIX>_NARRATION=json` (see
[flags and environment](flags-and-environment.md#which-setting-wins)),
stderr carries one JSON object per line: UTF-8, no indentation, no colour
codes, each line ended by `\n`. There is no live block. Data on stdout is
the same as in every other format. A prompt still waits for input, and
the stream reports it as a `prompt` line and the reply as an `answer`
line.
`app install jq@1.8.3 --narration json` prints:

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

## Events

Every event starts with `seq`, `time` and `event`, in that order. `seq`
is 1 on the first line and one more on each line after it, with no gaps.
`time` is RFC 3339 in UTC with six fractional digits and a `Z`, such as
`2026-10-20T04:34:56.944339Z`. The other keys follow in the order listed
here. The keys listed for a kind are always written, even when empty:
`version` is `""` when the program sets none, and `label`, `phase`,
`outcome` or `text` are `""` when the program passes an empty string. Only
`hint`, `task`, `partial` and `signal` are left out when they do not
apply.

| `event`  | `state`     | Keys after `event`, in order |
|----------|-------------|------------------------------|
| `begin`  |             | `app`, `version`, `schema` |
| `line`   |             | `level`, `text`, `hint` on an error line that has one, `task` when the line comes from a task |
| `task`   | `started`   | `task`, `state`, `label` |
| `task`   | `phase`     | `task`, `state`, `phase` |
| `task`   | `progress`  | `task`, `state`, `phase`, `done`, `total`, `unit` |
| `task`   | `done`      | `task`, `state`, `outcome`, `elapsed_ms` |
| `task`   | `failed`    | `task`, `state`, `outcome`, `elapsed_ms` |
| `task`   | `discarded` | `task`, `state` |
| `output` |             | `task` when a task wrote it, `stream`, `text`, `partial` only when true |
| `exit`   |             | `code`, `signal` when a signal ended the run |

| Field        | Values |
|--------------|--------|
| `schema`     | 1; it changes only with a breaking change |
| `level`      | `info`, `success`, `warn`, `error`, `hint`, `debug`, `prompt`, `answer` |
| `text`       | on a `line`, the message as plain output shows it without its prefix; an error starts with a capital letter |
| `task`       | a small integer, from 1 in start order, unique within the run |
| `done`, `total` | the progress counts; `total` is -1 when unknown |
| `unit`       | `items` or `bytes` |
| `outcome`    | the outcome line's text, without the duration |
| `elapsed_ms` | milliseconds since the task started, not rounded: `1607` where plain shows `(2s)` |
| `stream`     | `stdout` or `stderr` |
| `signal`     | `SIGINT` or `SIGTERM` |

JSON escapes every control character in a string, and writes `<`, `>`
and `&` as `\u003c`, `\u003e` and `\u0026`, as Go's JSON encoder does.
A multi-line error keeps its newlines in `text` as `\n`, while plain
output trims the last ones and indents the others.

## Start and end

- **`begin` is the first line and `exit` the last** on a normal run. The
  final error line, if any, comes before `exit`.
- **A failure before output is set up still ends in `exit`**: an error
  `line` and an `exit` with code 2, and no `begin`; see
  [flags and environment](flags-and-environment.md#errors-before-output-is-set-up).
- **A panic in the command still ends in `exit`**, code 2, before the
  crash report.
- **A stream without `exit` ended abnormally**: a second signal that
  could not finish the output, a crash outside the command, or a forced
  kill. A consumer must not read a missing `exit` as success.
- **A task with no `done`, `failed` or `discarded` at `exit` was
  interrupted.** clix makes up no outcome for it.

A run stopped with Ctrl-C ends each running task as `failed` with its
cancelled outcome, and `exit` names the signal.
`app build slow@1.0 --narration json`, stopped while compiling, prints:

<!-- capture: unchecked/sigint.jsonl -->
```text
{"seq":1,"time":"2026-10-20T02:55:15.919856Z","event":"begin","app":"app","version":"0.1.0","schema":1}
{"seq":2,"time":"2026-10-20T02:55:15.919956Z","event":"task","task":1,"state":"started","label":"Building slow 1.0"}
{"seq":3,"time":"2026-10-20T02:55:15.919975Z","event":"task","task":1,"state":"phase","phase":"compiling"}
{"seq":4,"time":"2026-10-20T02:55:16.217848Z","event":"task","task":1,"state":"failed","outcome":"Cancelled slow 1.0","elapsed_ms":297}
{"seq":5,"time":"2026-10-20T02:55:16.218021Z","event":"exit","code":130,"signal":"SIGINT"}
```

## Tasks and progress

A task sends `started`, then any number of `phase` and `progress`
events, then one of `done`, `failed` or `discarded`. Lines written
through the task carry its `task`; lines written after it ended are
dropped.

- **Progress is sampled about once per second**, on every tenth tick of
  a 100 ms clock, for each running task.
- **A `progress` event needs a phase and a progress text.** None is sent
  before the first phase, or for items or bytes with a total of 0.
- **A `progress` event is sent only on a change** of `phase`, `done`,
  `total` or `unit` since the last one for that task.
- **No last sample is forced** before `done` or `failed`, so the last may
  stop short: `seq` 4 above shows 3137536 of 4194304 bytes.
- **A new phase clears the progress** until the task reports it again.

`app build slow@1.0 --narration json` counts items:

<!-- capture: build-slow.jsonl -->
```text
{"seq":1,"time":"2026-10-20T04:34:06.721486Z","event":"begin","app":"app","version":"0.1.0","schema":1}
{"seq":2,"time":"2026-10-20T04:34:06.721594Z","event":"task","task":1,"state":"started","label":"Building slow 1.0"}
{"seq":3,"time":"2026-10-20T04:34:06.721612Z","event":"task","task":1,"state":"phase","phase":"compiling"}
{"seq":4,"time":"2026-10-20T04:34:07.723776Z","event":"task","task":1,"state":"progress","phase":"compiling","done":1,"total":2,"unit":"items"}
{"seq":5,"time":"2026-10-20T04:34:08.124054Z","event":"task","task":1,"state":"done","outcome":"Built slow 1.0","elapsed_ms":1402}
{"seq":6,"time":"2026-10-20T04:34:08.124190Z","event":"exit","code":0}
```

## Output events

- **A task's writes are reported, not passed through.** Bytes a task
  writes to either of its writers become `output` events with its `task`
  and never also reach stdout or stderr.
- **The program's own stderr writer becomes `output` events** with no
  `task`. Its stdout writer is data and passes through to stdout
  unchanged.
- **One event per line.** A line ends at `\n`, `\r`, or `\r\n` within one
  write, and the ending stays in `text`.
- **8 KiB without an ending** go as one event with `"partial":true`; the
  rest of the line follows in later events.
- **A last unended line goes when its task ends**, or just before `exit`
  for a task still running and for program-level output. After a task
  has ended, each write to its writers is sent at once.
- **Invalid UTF-8 is replaced** with U+FFFD, so a partial cut inside a
  character shows as a replacement character.
- **No event is sent after `exit`.** Later writes to stderr writers are
  dropped; the program's own stdout writer still passes through.

## Quiet and verbose

`--quiet` leaves out `started`, `phase`, `progress`, `done` and
`discarded` events, and `info`, `success` and standalone `hint` lines. A
failed task still sends `failed`, with no `started` before it; its
warnings, its `debug` lines under `--verbose` and its `output` events
keep its `task`.
`app install zlib@9.9 --quiet --narration json` prints:

<!-- capture: install-fail-quiet.jsonl -->
```text
{"seq":1,"time":"2026-10-20T02:55:05.354535Z","event":"begin","app":"app","version":"0.1.0","schema":1}
{"seq":2,"time":"2026-10-20T02:55:05.354642Z","event":"task","task":1,"state":"failed","outcome":"Failed to install zlib 9.9","elapsed_ms":0}
{"seq":3,"time":"2026-10-20T02:55:05.354656Z","event":"line","level":"error","text":"Download zlib 9.9: 404 Not Found"}
{"seq":4,"time":"2026-10-20T02:55:05.354675Z","event":"exit","code":1}
```

`--verbose` adds `debug` lines.

## Passthrough commands

A command that hands stdin, stdout and stderr to a child, such as
`app run -- make`, is exempt: the child writes straight to the terminal
or the pipe, and clix cannot turn its stderr into events. `begin` and
`exit` still mark the start and the end of the run.

## Rules for consumers

- **Consumers must tolerate lines that are not JSON**, such as a crash
  report, a passthrough child's output, or bytes a program writes to
  stderr without going through clix.
- **Consumers must ignore unknown events, states, values and fields.**
  Adding one is compatible; renaming or removing one raises `schema`.
- **Use `seq` for order and `exit` for the result.** `code` in `exit` is
  the process exit code; see [exit codes](exit-codes.md).

## Where to go next

- [Signals and exit](../guide/signals-and-exit.md): what ends a run and
  with which code.
- [Testing](../guide/testing.md): reading the stream back in a test.
- [Terminal](terminal.md): the same run on a terminal.
