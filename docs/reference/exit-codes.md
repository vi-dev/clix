# Exit codes

This page is for anyone who checks how a clix program ended: a script, a
CI step, an agent. It lists every exit code and when it is used. A
program can rely on all of it: a change to this page is a breaking
change. How a command chooses its code is in
[signals and exit](../guide/signals-and-exit.md).

## The codes

| Code  | When |
|-------|------|
| 0     | The command succeeded, including when there was nothing to do or when it finished without an error after a signal. |
| 1     | The command failed with an error, printed as the final `ERROR` line, or a batch failed on items it has already narrated. |
| 2     | The run failed before output was set up, printed as `Error: …` (in JSON as an error `line`), or the command panicked. |
| 130   | `SIGINT` (Ctrl-C) cancelled the run and the command returned an error, or a second signal was `SIGINT`. |
| 143   | `SIGTERM` cancelled the run and the command returned an error, or a second signal was `SIGTERM`. |
| 126   | Recommended for a command that hands the terminal to a child when it finds the child but cannot run it. |
| 127   | Recommended for such a command when it cannot find the child. |
| 128+n | Recommended for such a command when its child is killed by signal n. |
| other | A code the command chose, such as a child's own exit code passed on. |

## What each code means in detail

- **0.** A run with nothing to do is a success, and so is a command that
  sees a signal and still finishes cleanly.
- **1.** A cancellation with no signal behind it, such as a timeout the
  command set itself, is an ordinary error and exits 1. An error from the
  program's own setup also exits 1.
- **2.** The early failures are a bad `--color`, `--narration` or
  `<PREFIX>_NARRATION` value, a flag that cannot be parsed, an unknown
  command and arguments the command rejects; see
  [flags and environment](flags-and-environment.md#errors-before-output-is-set-up).
  After a panic, Go prints the stack and the process exits 2.
- **130 and 143.** These follow a signal only. clix prints no final error
  for the cancellation itself; each running task ends with its cancelled
  outcome. A real error found while stopping still prints as the final
  `ERROR` line, and the code stays 130 or 143. A second signal exits at
  once with the code for that second signal.
- **126, 127, 128+n and other codes.** clix exits with whatever code a
  command chooses and prints nothing, and that code wins over 130 and
  143. 126, 127 and 128+n are the shell's convention, recommended for a
  command that runs a child for the user; clix does not produce them
  itself. The demo's `app run` passes on a child's own exit code. A child
  that is not found gives an `ERROR` line and exit 1, and a child killed by
  a signal gives exit 255.

In JSON, the `exit` event carries the same code in `code`, and `signal`
names the signal that ended the run; see
[JSON events](json-events.md#start-and-end).

## Where to go next

- [Signals and exit](../guide/signals-and-exit.md): choosing a code and
  handling Ctrl-C.
- [Commands](../guide/commands.md): the batch and passthrough shapes.
