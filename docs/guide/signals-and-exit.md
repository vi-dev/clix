# Signals and exit

This page is for a developer deciding how a program ends. Afterwards you
can choose between `Run` and `Execute`, set an exit code, tell a signal
from other cancellations, and know what the user sees when a run is
stopped or panics.

The examples come from `app`, the demo program in
[examples/app/main.go](../../examples/app/main.go). The full list of
exit codes is in [exit codes](../reference/exit-codes.md).

## Run the program with Run

`clix.Run` is the whole of `main`. It runs the root command, turns the
outcome into an exit code and exits with it, and it watches for `SIGINT`
and `SIGTERM` while the command runs.

<!-- code: examples/app/main.go#main -->
```go
func main() {
    clix.Run(newRoot(), clix.Options{App: "app", Version: "0.1.0", EnvPrefix: "APP", Hint: hint})
}
```

## Run it yourself with Execute

`clix.Execute(ctx, root, opts)` does the same work and returns the code
and the error instead of exiting. It does not watch for signals: the
caller's context is the only way to cancel. Tests use it, see
[testing](testing.md#run-a-whole-command).

## Choose the exit code

A command that returns nil exits 0. One that returns an error gets that
error printed once, as the final `ERROR` line with its hint, and exits 1;
clix prints no usage text after it. A command's own check of its input
returns an ordinary error in the same way:

<!-- capture: install-nosuch.txt -->
```text
ERROR Package nosuch not found
  hint: Check the package name or run `app update`
---exit 1---
```

An error before the console exists exits 2: a bad `--color` or
`--narration`, a flag cobra cannot parse, or arguments that fail the
command's `Args` check. It is printed as `Error: …`, or, when JSON
narration was asked for, as an error `line` event and an `exit` event
with code 2. The full rule is in
[flags and environment](../reference/flags-and-environment.md).

`&clix.ExitError{Code: n}` exits with `n` and prints nothing, for a
command that has already said what went wrong. A batch returns it with
code 1 after its summary, see [commands](commands.md#run-one-verb-over-many-items).
A command that hands the terminal to a child returns the child's code,
as `runPassthrough` in [commands](commands.md#hand-the-terminal-to-a-child)
does.
`app run --color never -- sh -c 'echo out; echo err >&2; exit 3'` exits 3
with only the child's own output:

<!-- capture: run-passthrough.txt -->
```text
err
---stdout---
out
---exit 3---
```

## Stop on Ctrl-C

The first `SIGINT` cancels the command's context. Each running task ends
with its `Cancelled` outcome as an `ERROR` line, clix prints no final
error, and the run exits 130. In JSON the `exit` event names the signal.
`app build slow@1.0 --narration json`, stopped with Ctrl-C while
compiling, prints:

<!-- capture: unchecked/sigint.jsonl -->
```text
{"seq":1,"time":"2026-10-20T02:55:15.919856Z","event":"begin","app":"app","version":"0.1.0","schema":1}
{"seq":2,"time":"2026-10-20T02:55:15.919956Z","event":"task","task":1,"state":"started","label":"Building slow 1.0"}
{"seq":3,"time":"2026-10-20T02:55:15.919975Z","event":"task","task":1,"state":"phase","phase":"compiling"}
{"seq":4,"time":"2026-10-20T02:55:16.217848Z","event":"task","task":1,"state":"failed","outcome":"Cancelled slow 1.0","elapsed_ms":297}
{"seq":5,"time":"2026-10-20T02:55:16.218021Z","event":"exit","code":130,"signal":"SIGINT"}
```

A cancelled task is a `failed` event whose outcome is its `Cancelled`
text; the events are described in [JSON events](../reference/json-events.md).

## Stop on SIGTERM

`SIGTERM` does the same and exits 143. `app build slow@1.0 --color never`,
sent `SIGTERM` while compiling, prints:

<!-- capture: unchecked/sigterm.txt -->
```text
ERROR Cancelled slow 1.0
---exit 143---
```

## Tell a signal from other cancellations

When a signal cancels the context, its cause is a `clix.SignalError` that
holds the signal. A command that needs to know checks it with
`var se clix.SignalError; errors.As(context.Cause(ctx), &se)` and reads
`se.Sig`.

An error returned after a signal still prints as the final `ERROR` line
when it is not the cancellation itself, such as a checksum mismatch found
while stopping; the code stays 130 or 143. A command that finishes
without an error after a signal exits 0.

A cancellation with no signal behind it, such as a timeout the command
set itself, is an ordinary error: clix prints it as the final `ERROR`
line and exits 1.

## Press Ctrl-C twice

A second `SIGINT` or `SIGTERM` exits at once, with 130 or 143 for that
second signal. clix first makes one short attempt to finish the output:
it removes the live block and, in JSON, writes the `exit` event. It does
not wait for the command to return.

## Leave the terminal clean

On every exit clix removes a live block still on screen and keeps the
lines already printed, so the shell prompt starts on a clean line. How
the block is drawn and removed is in [terminal](../reference/terminal.md).

## Survive a panic

When a command panics, clix ends the narration with exit code 2: the live
block is removed and, in JSON, an `exit` event with `"code":2` closes the
stream. Then it panics again, so Go prints the stack and the process
exits 2.

## Send slog records to the console

`console.SlogHandler(c)` returns a `slog.Handler` that writes each record
through the console: `Warn` and above as a `WARN` line, anything lower as
a `DEBUG` line that only `--verbose` shows. Attributes follow the
message as `key=value`. Set it in `Options.Setup` with
`slog.SetDefault(slog.New(console.SlogHandler(c)))`, so library logs share
the user's terminal without breaking the live block or the JSON stream.

## Where to go next

- [Exit codes](../reference/exit-codes.md): every code and when it is
  used.
- [JSON events](../reference/json-events.md): the `exit` event and its
  fields.
- [Commands](commands.md): which shape ends with which code.
