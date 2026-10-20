# Flags and environment

This page is for anyone who runs a clix program or wraps one in a script.
It lists the flags and variables every clix program accepts, how they
combine, and what a bad value prints. A program can rely on all of it: a
change to this page is a breaking change.

The examples come from `app`, the demo program in
[examples/app/main.go](../../examples/app/main.go), whose variable
prefix is `APP`. In a capture, stdout follows a `---stdout---` line and a
last `---exit N---` line shows a status other than 0.

## The global flags

clix adds four flags to the root command, and every subcommand inherits
them. `app --help` prints:

<!-- capture: help.txt -->
```text
---stdout---
app installs packages from catalogs into the current project.

Usage:
  app [command]

Available Commands:
  build       Build a package from source
  check       Check packages (default: every package)
  clean       Remove unused package versions
  completion  Generate the autocompletion script for the specified shell
  env         Print the shell environment
  help        Help about any command
  info        Show a package
  install     Install a package
  publish     Publish the catalog to a registry
  run         Run a command in the package environment
  search      Search the catalogs
  status      Show the project packages
  sync        Install the locked packages
  update      Update the catalogs

Flags:
      --color string       colorize output: auto, always, or never (default "auto")
  -h, --help               help for app
      --narration string   narration format: auto, plain, or json (default "auto")
  -q, --quiet              suppress narration output
      --verbose            show debug output

Use "app [command] --help" for more information about a command.
```

### `--quiet`, `-q`

| Item      | Value |
|-----------|-------|
| Values    | none; the flag is a switch |
| Default   | off |
| Help text | `suppress narration output` |

`--quiet` hides `INFO`, `OK` and blank lines, hints that stand alone, task
start and end lines, and the live block. It keeps warnings, errors with
their hints, failed task outcomes, prompts and answers, and the child
output a task passes through its writers. `DEBUG` lines
follow `--verbose` only. Data on stdout does not change. The JSON stream
leaves out the same lines; see [JSON events](json-events.md#quiet-and-verbose).

### `--verbose`

| Item      | Value |
|-----------|-------|
| Values    | none; the flag is a switch |
| Default   | off |
| Help text | `show debug output` |

`--verbose` shows `DEBUG` lines, and `debug` lines in JSON. It works
together with `--quiet`.

### `--color`

| Item      | Value |
|-----------|-------|
| Values    | `auto`, `always`, `never` |
| Default   | `auto` |
| Help text | `colorize output: auto, always, or never` |

`always` and `never` decide colour on their own. `auto` defers to the
variables below, then colours when stderr is a terminal. Colour never
reaches stdout, and turning it off does not turn off the live block; see
[terminal](terminal.md#colour).

### `--narration`

| Item      | Value |
|-----------|-------|
| Values    | `auto`, `plain`, `json` |
| Default   | `auto` |
| Help text | `narration format: auto, plain, or json` |

| Value  | stderr is a terminal | Narration on stderr |
|--------|----------------------|---------------------|
| `auto` | yes                  | live: lines plus the live block |
| `auto` | no                   | plain |
| `plain`| any                  | plain: a static transcript |
| `json` | any                  | one JSON object per line |

Data on stdout is the same in every format. The live and plain forms are
in [terminal](terminal.md), the JSON stream in [JSON events](json-events.md).

### `--output`

| Item      | Value |
|-----------|-------|
| Values    | `text`, `json` |
| Default   | `text` |
| Help text | `output format: text or json` |

`--output` exists only on the commands that add it, and changes only
stdout. A bad value is an ordinary error: the final line is
`ERROR Unknown output format "yaml" (want text or json)` and the exit
code is 1.

## The variables

| Variable             | Effect when set |
|----------------------|-----------------|
| `NO_COLOR`           | `--color auto` means `never` |
| `FORCE_COLOR`        | `--color auto` means `always`, unless `NO_COLOR` is set |
| `<PREFIX>_NARRATION` | the value `--narration auto` uses: `auto`, `plain` or `json` |

A variable is set when its value is not empty; `NO_COLOR=` counts as
unset. `<PREFIX>_NARRATION` exists only when the program gives clix a
prefix, so `app` reads `APP_NARRATION`. An agent or a CI job sets it once
for every run.

## Which setting wins

Colour is decided in this order, and the first rule that applies wins:

1. `--color always` or `--color never`.
2. `NO_COLOR` set: no colour.
3. `FORCE_COLOR` set: colour.
4. Colour when stderr is a terminal.

`FORCE_COLOR` colours plain output; it does not draw a live block when
stderr is not a terminal.

For the narration format, a flag value other than `auto` wins and the
variable is not read at all, not even to check it. With `--narration
auto`, an unset `<PREFIX>_NARRATION` means `auto`, and any other value
is used as if it had been given to the flag.

## Bad values

A bad value for `--color`, `--narration` or `<PREFIX>_NARRATION` stops
the run before the command starts. clix checks `--color` first, then the
narration format. The run prints nothing on stdout and exits 2. On
stderr it prints one line in text, or two in JSON (an error `line` and
the `exit`):

| Setting              | Error text |
|----------------------|------------|
| `--color`            | `invalid --color value "X" (want auto, always, or never)` |
| `--narration`        | `invalid --narration value "X" (want auto, plain, or json)` |
| `<PREFIX>_NARRATION` | `invalid <PREFIX>_NARRATION value "X" (want auto, plain, or json)` |

`app --color bogus status` prints:

<!-- capture: bad-color.txt -->
```text
Error: invalid --color value "bogus" (want auto, always, or never)
---exit 2---
```

`app --narration xml status --color never` prints:

<!-- capture: bad-narration.txt -->
```text
Error: invalid --narration value "xml" (want auto, plain, or json)
---exit 2---
```

`APP_NARRATION=xml app status --color never` prints:

<!-- capture: bad-env-narration.txt -->
```text
Error: invalid APP_NARRATION value "xml" (want auto, plain, or json)
---exit 2---
```

## Errors before output is set up

Some errors happen before clix has set up its output: the bad values
above, a flag that cannot be parsed, an unknown command, and arguments
the command's argument check rejects. Each exits 2. An error from the
program's own setup comes later and is an ordinary `ERROR` line with
exit 1.

In text, the error is one line on stderr, `Error: ` followed by the
message as written, with no hint.

In JSON, the error is an error `line` event, its text starting with a
capital letter, followed by an `exit` event with code 2, and no `begin`.
JSON is used when the narration format resolves to `json`. When the
command line could not be parsed, names an unknown command or has
arguments the command rejects, clix looks at the arguments before any
`--` instead. There `--narration json` or `--narration=json` asks for
JSON, and a `--narration` with any other value but `auto` asks for text.
Otherwise `<PREFIX>_NARRATION=json` asks for JSON.
`APP_NARRATION=json app --color bogus status` prints:

<!-- capture: bad-env-narration.jsonl -->
```text
{"seq":1,"time":"2026-10-20T02:55:11.306530Z","event":"line","level":"error","text":"Invalid --color value \"bogus\" (want auto, always, or never)"}
{"seq":2,"time":"2026-10-20T02:55:11.306582Z","event":"exit","code":2}
```

## Where to go next

- [JSON events](json-events.md): the stream `--narration json` writes.
- [Terminal](terminal.md): the live block, plain output and colour codes.
- [Exit codes](exit-codes.md): every code and when it is used.
