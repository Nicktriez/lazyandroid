# lazyandroid

A terminal UI for the [`android` CLI](https://dl.google.com/android/cli/latest/linux_x86_64/install.sh)
and `adb`. It keeps the phones you have plugged in, your AVDs and your
environment visible, and streams long-running `android` commands — install, run,
build — instead of blocking a shell.

```
╭──────────────────────────────────────────────────────╮╭──────────────────────────────────────────────────────────────────╮
│ ● DEVICES (1)  AVDS (6)  APKS (3)                    ││ ENVIRONMENT                                                      │
│ ▸ Pixel 6a  34131JEGR08885                    ready  ││ sdk: /home/nicktriez/Android/Sdk                                 │
│                                                      ││ OUTPUT · install → Pixel 6a                                      │
│                                                      ││ $ android install --apks=… --device=34131JEGR08885              │
│                                                      ││ Standard Install: Sent 62.27 MB (1 APK)                         │
│                                                      ││ ✓ install → Pixel 6a finished                                   │
╰──────────────────────────────────────────────────────╯╰──────────────────────────────────────────────────────────────────╯
```

## Requirements

- Go 1.24 or newer (pinned to 1.27.1 via `.mise.toml`).
- The `android` CLI on `PATH` (`mise` manages it as `android-cli`).
- `adb`, from the SDK's `platform-tools`. It is rarely on `PATH` — the SDK puts
  it next to the SDK root — so lazyandroid looks where the `android` CLI says
  the SDK is, then `$ANDROID_HOME`, `$ANDROID_SDK_ROOT` and `~/Android/Sdk`,
  and only then `PATH`. `$ANDROID_ADB` overrides the search. With no adb at all
  the DEVICES list says so, rather than looking like nothing is plugged in.

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
| `tab` | switch the left list: DEVICES / AVDS / APKS — the `●` list is the target |
| `↑`/`↓`, `k`/`j` | move in the visible list |
| `I` | install the selected APK on the target device or booted AVD |
| `R` | `android run` — build, deploy and launch on the target device or booted AVD |
| `s` | start selected AVD (`android emulator start <id>`) |
| `x` | stop selected AVD (`android emulator stop <id>`) |
| `r` | refresh devices, APKs, AVDs and environment |
| `i` | refresh environment only |
| `p` | stream `android sdk list --all` |
| `d` | stream `android describe` |
| `esc` | cancel the running job |
| `pgup`/`pgdn`, `end` | scroll the output pane |
| `?` | toggle help |
| `q`, `ctrl+c` | quit (cancels a running job) |

One job runs at a time. Jobs are killed through the Go context passed to
`exec.CommandContext`, so `esc` and `q` actually terminate the child process.

## Devices over USB

The `android` CLI manages AVDs; it cannot enumerate the phones you have plugged
in. `adb devices -l` can, so that is what the DEVICES list shows, with the state
adb reports:

| State | Meaning |
| --- | --- |
| `ready` | adb reports `device` — installable and runnable |
| `unauthorized` | the phone has not accepted this host's key: unlock it and accept the USB debugging prompt |
| `offline` | adb sees it, but it is not answering |
| `no permissions` | a Linux udev rules problem |

Rows are coloured by state: green when the device can be targeted (`ready`, and
`running` for a booted AVD), yellow while it is seen but not usable yet
(`offline`, `booting`), red when the host has to fix something (`unauthorized`,
`no permissions`).

`I` and `R` target the list marked `●` in the pane title — the list you last
tabbed to. On DEVICES that is the device under the cursor, passed to
`--device=<serial>`; on AVDS it is the selected AVD, provided it is booted, and
the serial the CLI reports for it (`emulator-5554`) is passed the same way. So a
phone and a running emulator can be attached at once and the choice is explicit:
tab to AVDS to deploy to the emulator, and tab back to DEVICES to deploy to the
phone. Tabbing on to APKS to pick an APK leaves the target alone — otherwise the
phone plugged in for charging would silently take over the deploy. Selecting an
AVD that is not booted is refused with "press s to start it" rather than falling
back to the phone. A device that is not `ready` is refused with the reason,
rather than handed to adb to fail further down.

`I` also needs an APK, and takes the one selected in the APKS list. That list is
a scan of the current directory for `*.apk` — newest Gradle output first, caches
skipped — so the usual loop is: build in your project, run lazyandroid from that
project, `tab` to APKS, `I`. With no APK selected `R` passes no `--apks` and
lets the CLI decide what to build.

`android install` exits `0` even when it prints an error and installs nothing,
so the missing pieces are refused up front instead of being trusted to the exit
status: read the OUTPUT pane, not only the `✓`.

## Architecture

```
main.go                     entry point; wires the runner into the UI
internal/android/           CLI adapters
  runner.go                 Output() for quick commands, Stream() for line-by-line output
  client.go                 typed calls: Emulators, Info, SDKPackages
  device.go                 adb: the device list, `adb devices -l` parsing, finding adb
  project.go                APK discovery under the current directory
  args.go                   --device flag helpers
  text.go                   control-character stripping for the panes
  parse.go                  parsers for `emulator list --long` and `info`
  testdata/                 real CLI output used as parser fixtures
internal/ui/                Bubble Tea model, view, styles
  model.go                  state, messages, key handling, job runner, install/run
  view.go                   pane layout, the three target lists, windowing, truncation
```

The `android` CLI has no machine-readable output mode (`--json` is rejected),
so `internal/android/parse.go` parses the human-readable formats. The
`emulator list --long` parser derives column offsets from the header row rather
than hard-coding widths, and a golden test against captured CLI output fails
loudly if the format shifts.

The CLI cannot enumerate physical devices at all, so device discovery goes
through `adb devices -l`, parsed by `internal/android/device.go`.

## Known limitations

- Output lines wider than the pane are truncated with `…`. `android sdk list
  --all` is wider than most terminals; scroll with `pgup`/`pgdn`.
- Only the tail of the output is rendered (last 2000 lines are kept).
- Control characters are stripped as lines arrive, so a progress counter that
  animates in place (`android install`) collapses onto one line instead of
  repainting the pane next to it.
- The APK scan covers the current directory, up to 12 levels deep, and lists at
  most 40 files. Build first: Gradle writes APKs under
  `*/build/outputs/apk/`.
- `R` shells out to `android run`, which builds the project itself and needs
  whatever that needs (a Gradle wrapper, a JDK).
- Not wired up yet: `android create`, `android layout` (JSON UI tree),
  `android screen capture`, logcat.

## Tests

```sh
go test ./...
```
