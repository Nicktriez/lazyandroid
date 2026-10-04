# lazyandroid

A terminal UI for the [`android` CLI](https://dl.google.com/android/cli/latest/linux_x86_64/install.sh).
It keeps your AVDs and environment visible and streams long-running `android`
commands instead of blocking a shell.

```
╭──────────────────────────────────────────────────────╮╭──────────────────────────────────────────────────────────────────────────────────╮
│ EMULATORS  (6)                                       ││ ENVIRONMENT                                                                      │
│ ▸ medium_phone                   android-36  offline ││ sdk: /home/nicktriez/Android/Sdk                                                 │
│   small_phone                  android-36.1  offline ││ version: 1.0.16261425                                                            │
│   medium_tablet                  android-35  offline ││ OUTPUT                                                                           │
╰──────────────────────────────────────────────────────╯╰──────────────────────────────────────────────────────────────────────────────────╯
```

## Requirements

- Go 1.24 or newer (pinned to 1.27.1 via `.mise.toml`).
- The `android` CLI on `PATH` (`mise` manages it as `android-cli`).

## Build and run

```sh
mise install                 # Go toolchain for this project
go build -o bin/lazyandroid .
./bin/lazyandroid
```

`LAZYANDROID_BIN` overrides which executable is invoked, which is handy for
pointing at a stub during development.

## Keys

| Key | Action |
| --- | --- |
| `↑`/`↓`, `k`/`j` | move AVD selection |
| `s` | start selected AVD (`android emulator start <id>`) |
| `x` | stop selected AVD (`android emulator stop <id>`) |
| `r` | refresh AVD list and environment |
| `i` | refresh environment only |
| `p` | stream `android sdk list --all` |
| `d` | stream `android describe` |
| `esc` | cancel the running job |
| `pgup`/`pgdn`, `end` | scroll the output pane |
| `?` | toggle help |
| `q`, `ctrl+c` | quit (cancels a running job) |

One job runs at a time. Jobs are killed through the Go context passed to
`exec.CommandContext`, so `esc` and `q` actually terminate the child process.

## Architecture

```
main.go                     entry point; wires the runner into the UI
internal/android/           CLI adapter
  runner.go                 Output() for quick commands, Stream() for line-by-line output
  client.go                 typed calls: Emulators, Info, SDKPackages
  parse.go                  parsers for `emulator list --long` and `info`
  testdata/                 real CLI output used as parser fixtures
internal/ui/                Bubble Tea model, view, styles
  model.go                  state, messages, key handling, job runner
  view.go                   pane layout, list windowing, truncation
```

The `android` CLI has no machine-readable output mode (`--json` is rejected),
so `internal/android/parse.go` parses the human-readable formats. The
`emulator list --long` parser derives column offsets from the header row rather
than hard-coding widths, and a golden test against captured CLI output fails
loudly if the format shifts.

## Known limitations

- Output lines wider than the pane are truncated with `…`. `android sdk list
  --all` is wider than most terminals; scroll with `pgup`/`pgdn`.
- Only the tail of the output is rendered (last 2000 lines are kept).
- Not wired up yet: `android create`, `android layout` (JSON UI tree),
  `android screen capture`, logcat.

## Tests

```sh
go test ./...
```
