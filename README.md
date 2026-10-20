# clix

clix gives a Go command-line program built on
[cobra](https://github.com/spf13/cobra), the Go command-line library, one
consistent way to talk to the user. Data goes to stdout. Narration
(progress, outcomes, warnings, errors and hints) goes to stderr, as a live
block on a terminal, plain lines in a log, or one JSON object per line for
a script. clix adds the global flags, builds the console, turns errors and
signals into exit codes, and ships a test package. The `console` package
uses only the standard library and works without cobra.

```text
$ go get github.com/vi-dev/clix
```

## A few lines of main

This is the main of `hello`, the program that
[getting started](docs/getting-started.md) builds.

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

`clix.Run` runs the command, prints a final error if there is one, and
exits with the right code.

## What the user sees

`hello greet world --color never` prints:

<!-- capture: hello-greet.txt -->
```text
INFO Hello, world
OK Greeted world
```

The same run with `--narration json` prints one JSON object per line:

<!-- capture: hello-greet.jsonl -->
```text
{"seq":1,"time":"2026-10-20T02:55:15.572500Z","event":"begin","app":"hello","version":"0.1.0","schema":1}
{"seq":2,"time":"2026-10-20T02:55:15.572730Z","event":"line","level":"info","text":"Hello, world"}
{"seq":3,"time":"2026-10-20T02:55:15.572741Z","event":"task","task":1,"state":"started","label":"Greeting world"}
{"seq":4,"time":"2026-10-20T02:55:15.572754Z","event":"task","task":1,"state":"phase","phase":"thinking"}
{"seq":5,"time":"2026-10-20T02:55:15.873788Z","event":"task","task":1,"state":"done","outcome":"Greeted world","elapsed_ms":301}
{"seq":6,"time":"2026-10-20T02:55:15.874018Z","event":"exit","code":0}
```

## Flags

Every command gets four global flags, and a command that prints data can
add a fifth:

- `--color auto|always|never` turns colour on or off.
- `--narration auto|plain|json` picks the narration format.
- `-q`, `--quiet` keeps only warnings and errors.
- `--verbose` adds debug lines.
- `--output text|json` picks the data format, for a command that adds it.

## Where to go next

- [Getting started](docs/getting-started.md): build a small CLI step by step.
- [The guide](docs/guide/): narration, tasks, data, commands, testing, signals and exit.
- [The reference](docs/reference/): flags, environment, JSON events, terminal, exit codes.
- [Conventions](docs/conventions.md): the house style for what a command prints.

## License

MIT, see [LICENSE](LICENSE).
