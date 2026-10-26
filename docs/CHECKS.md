# Checks

Every check is read-only and runs without administrator rights. Each finding carries a
weight; the score is `100 - sum(weights)`, with each finding capped at 20 and the total
at 100.

| ID | Severity | Weight | What it means |
| --- | --- | --- | --- |
| `path.length` | warn / bad | 8 (>2,047 chars) / 20 (>8,191) | Some installers and legacy tools truncate `PATH` at 2,047 characters and silently drop entries. The Windows environment block caps at 32,767. |
| `path.dead` | warn | 2 per entry, max 10 | The directory no longer exists. Every command lookup walks it for nothing, and it hides which tools are actually installed. |
| `path.relative` | warn | 6 | A relative entry (`.\bin`, `.`) resolves against the current directory, so a file dropped there can run instead of the intended tool. |
| `path.duplicates` | info | 1 per entry, max 3 | The same directory is listed twice. Harmless, but it inflates `PATH` and slows resolution. |
| `tools.store-stub` | warn | 8 | The command resolves to a zero-byte `WindowsApps` alias. With no Store package installed, running it opens the Microsoft Store instead of the tool. |
| `tools.multiple` | info | 4 per tool family, max 12 | The same tool is installed in more than one place, so which version you get depends on `PATH` order. Tool families are reported once (`java` and `javac` are one JDK). |
| `system.longpaths` | warn | 8 | `HKLM\SYSTEM\CurrentControlSet\Control\FileSystem\LongPathsEnabled` is 0. Paths past 260 characters fail in tools that honour the legacy limit. |
| `git.identity` | warn | 6 | Global `user.name` or `user.email` is missing, so new repositories cannot commit until one is set. |
| `git.lineendings` | info | 1 | `core.autocrlf` is unset on Windows, the usual cause of whole-file diffs after a checkout. |
| `git.longpaths` | info | 1 | `core.longpaths` is off, so git itself is subject to the 260-character limit. |
| `ports.occupied` | info | 1 per port, max 3 | Something is listening on a common dev port. Expected with a server running; suspicious after a crash. |
| `caches.size` | info / warn | 2 (>5 GB) / 6 (>20 GB) | Reclaimable package-manager cache. Not a fault, just disk you are not using. |

## What is deliberately not checked

- **Installed updates, drivers, services and startup entries.** Different problem, different
  tool, and the answers go stale the moment Windows updates.
- **Anything that requires elevation to read.** A check that fails silently under a normal
  shell is worse than no check.
- **Anything that phones home.** There is no network access in the binary at all.
- **Security posture.** `envdoctor` reports developer-environment hygiene, not whether a
  machine is compromised. Do not read an `A` as "safe".

## Tool families

`python`, `python3`, `py`, `pip` and `pip3` are one family; `node`, `npm`, `npx`, `pnpm`,
`yarn` and `bun` are another; `java`, `javac` and `jar` are another. Family grouping keeps
one installation from being reported five times because it ships five command names.

Installation roots are collapsed too: a git install that exposes both `bin\` and `cmd\`
counts once, while JDK 17 and JDK 21 remain separate, because that distinction actually
changes which compiler runs.

## Adding a check

1. Return one or more `Finding` values with a stable `ID` of the form `<category>.<name>`.
2. Pick the weight from the table above, or add a row here first.
3. Add a test for the pure part of the logic.
4. Keep it read-only. If a check needs a shell command, it must not change state.
