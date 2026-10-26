# envdoctor

**A health check for the machine you build on.** One command, read-only, no admin rights.

`envdoctor` inspects a Windows developer setup and prints what is actually wrong with it:
the Microsoft Store stub that hijacks `python`, dead `PATH` entries, three Node installs
fighting over which version runs, long-path support turned off, gigabytes of stale caches.
It changes nothing, needs no elevation and never touches the network. Think
`flutter doctor`, for a Windows box.

Every finding comes with a one-line fix, and the score is just the sum of what is wrong —
so nothing is hidden behind a number.

![envdoctor demo](docs/demo.gif)

*The recording above is the real output of `envdoctor --no-color`, captured straight into
`docs/demo.gif` by [`tools/make-demo.ps1`](tools/make-demo.ps1). Nothing is mocked up.*

```
envdoctor 0.1.1
Windows 11 Home 25H2 (build 26200.9711) · amd64

  53 / 100  grade F
  This setup is actively costing you time.

  ! Git identity is incomplete
      Missing global setting(s): user.email, user.name. New commits will be rejected or
      attributed to a placeholder.
      fix: set user.name and user.email with git config --global.

  ! 2 PATH entries missing from disk
      Missing: ...\.dotnet\tools, ...\go\bin. Every command lookup walks these for nothing,
      and they hide which tools are really installed.
      fix: Delete them from PATH (System Properties > Environment Variables).

  ! Long path support is off
      LongPathsEnabled=0. Checkouts and builds fail once a nested path passes 260
      characters, which deep node_modules and nested build folders reach easily.

  ! 1 tool shadowed by Microsoft Store aliases
      Python: py, python, python3 (alias only). These are zero-byte WindowsApps aliases:
      with no Store package installed they open the Microsoft Store instead of running.

  - 10.9 GB of package caches
      10.9 GB across 4 locations. Top: pip 6.7 GB, npm 2.5 GB, go 940.2 MB, uv 761.2 MB.

  - 2 tools installed in more than one place
      Java: ...\jdk-17.0.19.10-hotspot\bin, ...\jdk-21.0.12.101-hotspot\bin;
      Node: ...\Program Files\nodejs, ...\nvm\.nodejs, ...\hermes\node

  1 of 6 checks clean · 10.9 GB reclaimable
```

That output is from a real machine — the author's. The score is deliberately unkind:
every line above is a concrete thing that breaks builds or wastes an hour.

## Install

**Download the binary:** grab `envdoctor.exe` from
[Releases](https://github.com/sinnerk/envdoctor/releases) and put it anywhere on `PATH`.
It is a single static binary with no runtime and no dependencies. It is not code-signed,
so SmartScreen may warn the first time.

**With Go** (1.24 or newer):

```powershell
go install github.com/sinnerk/envdoctor/cmd/envdoctor@latest
```

`go install` puts the binary in `%USERPROFILE%\go\bin`. The Go installer adds that folder
to `PATH`; if it is missing from yours, add it. To build from source instead:

```powershell
git clone https://github.com/sinnerk/envdoctor.git
cd envdoctor
go build -o envdoctor.exe ./cmd/envdoctor
```

## Use

```powershell
envdoctor                     # human-readable audit
envdoctor --json              # machine-readable, every finding and weight
envdoctor --json --redact     # safe to paste into an issue
envdoctor --fast              # skip the cache-size walk
envdoctor --fail-under 80     # exit 1 when the score drops below 80 (CI gate)
```

| Flag | Effect |
| --- | --- |
| `--json` | Print the full report as JSON. |
| `--redact` | Replace `%USERPROFILE%`, `%LOCALAPPDATA%`, `%APPDATA%`, `%ProgramFiles%` with variable names before printing. |
| `--fast` | Skip the package-cache size scan, which is the only slow check (about 2 s on multi-gigabyte caches). |
| `--fail-under N` | Exit with status 1 when the score is below `N`. Default 0, never fails. |
| `--no-color` | Disable ANSI colour (also honours `NO_COLOR`). |

Exit codes: `0` clean run, `1` below `--fail-under`, `2` bad usage.

## What it checks

| Check | What it finds |
| --- | --- |
| **PATH health** | Length past the 2,047-character limit where installers truncate it, entries that point at deleted directories, directories listed twice, relative entries that can execute a file from your current folder. |
| **Store aliases** | `python`, `py`, `node` and friends resolving to zero-byte `WindowsApps` stubs that open the Microsoft Store instead of running. |
| **Duplicate installs** | One tool family installed in several places (`node` from Program Files, nvm and a bundled runtime), reported once per family rather than once per command name. |
| **Long paths** | `LongPathsEnabled=0`, which breaks checkouts and builds past 260 characters — the classic deep `node_modules` failure. |
| **Caches** | Reclaimable GB in npm, pip, uv, Go, NuGet, Cargo, Gradle, Maven and VS Code caches. |
| **Dev ports** | Loopback ports (3000, 5173, 5432, …) still held by a crashed dev server or database. |
| **Git** | Missing global `user.name`/`user.email`, unset `core.autocrlf`, and `core.longpaths`. |
| **Host** | Edition, build and architecture, with Windows 11 detected from the build number. |

[`docs/CHECKS.md`](docs/CHECKS.md) documents each check, its weight and how to fix it.

## How the score works

Start at 100 and subtract the weight of every finding. Each finding is capped at 20
points and the total at 100, so one badly broken thing cannot mask the rest.

| Grade | Score | Meaning |
| --- | --- | --- |
| A | 90–100 | Clean |
| B | 80–89 | Mostly clean |
| C | 70–79 | Workable, with real risks |
| D | 60–69 | Messy |
| F | 0–59 | Actively costing you time |

The tool is strict on purpose. A pristine CI image usually lands around 70, because its
`PATH` really is 2,700 characters and carries dead entries from preinstalled SDKs. An `A`
means there is nothing to fix, not that no warnings happened to fire.

## Use it in CI

`--json` plus `--fail-under` turns the audit into a gate, which is useful on a
self-hosted Windows runner that drifts over time:

```yaml
- name: Dev environment audit
  run: |
    go run github.com/sinnerk/envdoctor/cmd/envdoctor@latest --json --redact --fail-under 80
```

## Design rules

- **Read-only.** Nothing is created, deleted or modified. Fixes are printed, not applied.
- **No admin rights.** Registry reads, PATH reads, cache sizes, loopback dials.
- **No network.** The binary never makes a request, so it works offline and cannot phone home.
- **No telemetry, no config file, no state.** One static binary, deterministic output.
- **Explainable score.** Every point lost traces to a named finding with a documented weight.

## Roadmap

- `--fix` with a dry-run diff, applying only reversible changes (dead PATH entries, cache prune).
- Native checks for macOS and Linux once the Windows set is solid.
- Profile scanners: shell startup cost, VS Code extension count, WSL disk usage.

## License

MIT. See [LICENSE](LICENSE).
