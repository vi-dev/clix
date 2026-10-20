# Decisions

This page is for anyone who wants to know why clix behaves as it does.
Each entry states one choice and the reason for it; the page it links to
documents the behaviour itself.

**Narration goes to stderr and data to stdout.** clix keeps the two
apart because a script reads stdout: `app status > out.txt` leaves
only the result in the file, whatever the program narrates while it
runs. See [data](guide/data.md#keep-stdout-for-the-result) and
[conventions](conventions.md#keep-the-two-planes-apart).

**JSON narration is one object per line.** clix writes each event as one
newline-ended JSON object because a consumer can then read the stream a
line at a time, act on each event as it arrives, and skip a line that is
not JSON. See [JSON events](reference/json-events.md#the-stream).

**Progress is sampled about once a second.** clix sends a `progress`
event only on every tenth tick of its 100 ms clock, and only when the
value changed, because a fast download would otherwise flood the stream
with events no reader needs. See
[JSON events](reference/json-events.md#tasks-and-progress).

**One lock guards the console, and progress is kept in atomic counters.**
Every line, redraw and event is written under the same lock, so output
from different goroutines never interleaves. Reporting progress takes
no lock, because it runs inside copy loops that call it for every chunk
and has to stay cheap. See [tasks](guide/tasks.md#report-progress).

**The cursor is never hidden.** clix leaves the cursor alone because a
program that dies before it can restore the cursor would leave the
user's terminal without one. Each redraw is a single write, so the block
is never seen half drawn. See [terminal](reference/terminal.md#the-live-block).

**Width is counted in characters.** clix counts one column per
character because that needs no table of character widths and is right
for the labels programs usually print. Wide and combining characters can
misalign the block, a known limit. See [terminal](reference/terminal.md#width).

**Exit codes 130 and 143 follow only a signal.** clix exits 130 after
`SIGINT` and 143 after `SIGTERM` because shells and CI read those codes
as "stopped by a signal". A cancellation with no signal behind it, such
as a timeout, is an ordinary error and exits 1, so a script can tell the
two apart. See
[signals and exit](guide/signals-and-exit.md#tell-a-signal-from-other-cancellations)
and [exit codes](reference/exit-codes.md#the-codes).

**Exit removes the live block.** clix clears a block still on screen on
every exit, a panic and a second signal included, because a frozen
spinner line left above the shell prompt looks like work still running.
Lines already printed stay. See
[signals and exit](guide/signals-and-exit.md#leave-the-terminal-clean).

**In JSON mode a task's writes are buffered per line, up to 8 KiB.**
clix turns each line a task writes into one `output` event because a
consumer wants whole lines, not the chunks a child happens to write. A
line longer than 8 KiB goes out in parts marked `partial`, so one
endless line cannot hold memory or delay the stream. See
[JSON events](reference/json-events.md#output-events).

**Every example on these pages comes from a committed demo program.**
The code blocks are functions in [examples](../examples/) and the
transcripts are output the demo recorded into [captures](captures/),
because typed examples drift from what the code does. `make docs-check`
runs the [check tool](tools/check/main.go), which fails when a page
differs from its source or a link does not resolve.

**Recording the examples again is followed by updating the pages.**
The recorded `.jsonl` files keep real times, so `make docs-captures`
changes them on every run. `make docs-check` then names each page that
quotes a changed file, and those pages are updated to match. A capture
added later is recorded with `docs/tools/capture.sh --checked-only` into
a temporary directory and only the new files are copied in, so the
existing captures keep their times.
