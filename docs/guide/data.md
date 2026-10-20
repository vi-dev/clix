# Data

This page is for a developer whose commands print a result: a table, JSON,
key/value lines or a script. Afterwards you can print each of them, offer
`--output json`, and keep stdout free of anything else.

Data is what a script reads, and it goes to stdout. It is never coloured
and `--quiet` leaves it alone. Everything else is narration on stderr, see
[narration](narration.md). The examples come from `app`, the demo program
in [examples/app/main.go](../../examples/app/main.go). In a capture,
stderr comes first and stdout follows the `---stdout---` line.

## Keep stdout for the result

A command can end with both: the data on stdout, then one summary line on
stderr. A dry-run plan is data too, so a script can read it.

<!-- code: examples/app/main.go#publishCatalog -->
```go
func publishCatalog(cmd *cobra.Command, args []string) error {
    c := clix.ConsoleOf(cmd)
    ref := args[0]
    t := c.Start("Comparing with " + ref)
    t.Phase("comparing")
    for i := range 42 {
        t.Progress(int64(i+1), 42, console.Items)
    }
    t.Done("Compared with " + ref + ": 1 to push, 41 unchanged")
    if publishDryRun {
        c.Table([]string{"package", "version", "status"}, [][]string{{"jq", "1.8.3", "updated"}})
        c.Successf("Would publish %s: 1 to push, 41 unchanged", ref)
        return nil
    }
    if err := pushPackage(cmd.Context(), c, "jq 1.8.3"); err != nil {
        return err
    }
    c.Successf("Published %s: 1 pushed, 41 unchanged", ref)
    return nil
}
```

`app publish registry.example/cat --dry-run --color never > plan.txt`
leaves only the table in `plan.txt`, because both `OK` lines went to
stderr:

<!-- capture: publish-dry-run.txt -->
```text
OK Compared with registry.example/cat: 1 to push, 41 unchanged
OK Would publish registry.example/cat: 1 to push, 41 unchanged
---stdout---
PACKAGE  VERSION  STATUS
jq       1.8.3    updated
```

## Print a table

`Table(headers, rows)` writes the headers in capitals and lines up each
column two spaces from the next, with no borders and no trailing spaces.
Put `-` in a cell with no value.

<!-- code: examples/app/main.go#printStatus -->
```go
func printStatus(cmd *cobra.Command, args []string) error {
    c := clix.ConsoleOf(cmd)
    type row struct {
        Name      string `json:"name"`
        Version   string `json:"version"`
        Installed bool   `json:"installed"`
        Catalog   string `json:"catalog,omitempty"`
    }
    rows := []row{{"jq", "1.8.3", true, "main"}, {"zlib", "1.3.1", false, ""}}
    if *statusOutput == "json" {
        return c.JSON(rows)
    }
    cells := [][]string{}
    for _, r := range rows {
        installed, catalog := "no", r.Catalog
        if r.Installed {
            installed = "yes"
        }
        if catalog == "" {
            catalog = "-"
        }
        cells = append(cells, []string{r.Name, r.Version, catalog, installed})
    }
    c.Table([]string{"package", "version", "catalog", "installed"}, cells)
    c.BlankErr()
    c.Warn("1 package not installed — run `app sync`")
    return nil
}
```

`BlankErr` puts one blank line on stderr before the warning, and none
under `--quiet`. `app status --color never` prints:

<!-- capture: status.txt -->
```text

WARN 1 package not installed — run `app sync`
---stdout---
PACKAGE  VERSION  CATALOG  INSTALLED
jq       1.8.3    main     yes
zlib     1.3.1    -        no
```

An empty result prints no table, not even the headers; the command says so
on stderr. `app search x --color never` prints:

<!-- capture: search-none.txt -->
```text
WARN No packages match "x"
```

## Offer JSON with --output

`clix.OutputFlag(cmd)` adds `--output text|json` to one command and
returns where its value lands. A value other than `text` or `json` fails
before the command runs.

<!-- code: examples/app/main.go#statusCommand -->
```go
func statusCommand() *cobra.Command {
    cmd := &cobra.Command{Use: "status", Short: "Show the project packages", Args: cobra.NoArgs, RunE: printStatus}
    statusOutput = clix.OutputFlag(cmd)
    return cmd
}
```

`JSON(v)` writes one document with a two-space indent and a final newline.
Scripts rely on its field names, so they stay the same from one release to
the next. A field with no value is left out, as `catalog` is for zlib.
It holds only the result rows, never narration.
`app status --output json` prints:

<!-- capture: status-json.txt -->
```text
---stdout---
[
  {
    "name": "jq",
    "version": "1.8.3",
    "installed": true,
    "catalog": "main"
  },
  {
    "name": "zlib",
    "version": "1.3.1",
    "installed": false
  }
]
```

When the command already has a variable for the value,
`clix.AddOutputFlag(cmd, &v)` adds the flag and `clix.ValidateOutput(v)`
checks it. The error text and exit code for a bad value are in
[flags and environment](../reference/flags-and-environment.md).

## Print key/value lines

`Data(format, a...)` writes text as it is given. Here `printInfo` pads
each key with its colon to twelve columns with `%-12s` and leaves out a
field with no value, as `license` is here.

<!-- code: examples/app/main.go#printInfo -->
```go
func printInfo(cmd *cobra.Command, args []string) error {
    c := clix.ConsoleOf(cmd)
    fields := [][2]string{
        {"name", args[0]},
        {"version", "1.8.3"},
        {"homepage", "https://jqlang.org"},
        {"license", ""},
        {"catalog", "main"},
    }
    for _, f := range fields {
        if f[1] != "" {
            c.Data("%-12s%s\n", f[0]+":", f[1])
        }
    }
    return nil
}
```

`app info jq --color never` prints:

<!-- capture: info.txt -->
```text
---stdout---
name:       jq
version:    1.8.3
homepage:   https://jqlang.org
catalog:    main
```

## Stream rows as they come

`TableWriter` takes the columns up front, so each `Row` is written as soon
as it is known. A `Column` has a `Width`, and a longer cell is cut short
with `…`; the last column may have none. `Align: console.Right` lines a
column up on its right edge. The headers are written with the first row,
so no row means no table.

<!-- code: examples/app/main.go#searchPackages -->
```go
func searchPackages(cmd *cobra.Command, args []string) error {
    c := clix.ConsoleOf(cmd)
    w := c.TableWriter(
        console.Column{Name: "package", Width: 8},
        console.Column{Name: "size", Width: 6, Align: console.Right},
        console.Column{Name: "summary"},
    )
    found := 0
    for _, p := range catalogPackages {
        if !strings.Contains(p.Name, args[0]) {
            continue
        }
        found++
        if *searchOutput == "json" {
            if err := c.JSONLine(p); err != nil {
                return err
            }
            continue
        }
        w.Row(p.Name, p.Size, p.Summary)
    }
    if found == 0 {
        c.Warnf("No packages match %q", args[0])
    }
    return nil
}
```

`app search j --color never` prints:

<!-- capture: search.txt -->
```text
---stdout---
PACKAGE     SIZE  SUMMARY
jq          1 MB  Command-line JSON processor
jless       2 MB  Command-line JSON viewer
jsonnet…    3 MB  Package manager for Jsonnet
```

`JSONLine(v)` writes one JSON object per line, for a script that reads the
rows as they come. `app search j --output json` prints:

<!-- capture: search-json.txt -->
```text
---stdout---
{"name":"jq","size":"1 MB","summary":"Command-line JSON processor"}
{"name":"jless","size":"2 MB","summary":"Command-line JSON viewer"}
{"name":"jsonnet-bundler","size":"3 MB","summary":"Package manager for Jsonnet"}
```

## Print a script

A script for the shell to run is data, and nothing else may share its
stream.

<!-- code: examples/app/main.go#printEnv -->
```go
func printEnv(cmd *cobra.Command, args []string) error {
    c := clix.ConsoleOf(cmd)
    c.Data("export PATH=\"$HOME/.app/packages/jq/1.8.3/bin:$PATH\"\n")
    c.Data("export APP_ACTIVE=1\n")
    return nil
}
```

`app env --color never` writes nothing to stderr, so
`eval "$(app env)"` reads only the script:

<!-- capture: env.txt -->
```text
---stdout---
export PATH="$HOME/.app/packages/jq/1.8.3/bin:$PATH"
export APP_ACTIVE=1
```

## Leave a blank line

`Blank` writes one empty line on stdout, for a gap between two tables.
`BlankErr` writes one on stderr, as in `status` and in the prompt of
[narration](narration.md#ask-a-question), and none under `--quiet`.

## Where to go next

- [Commands](commands.md): queries, dry runs and the other command shapes.
- [Testing](testing.md): assert on what a command writes to stdout.
- [Flags and environment](../reference/flags-and-environment.md):
  `--output` and the global flags.
