# WinGet manifests

These are the manifests for submitting `envdoctor` to the
[Windows Package Manager community repository](https://github.com/microsoft/winget-pkgs).
They are kept here so a release can be packaged without regenerating them from scratch.

Once the submission below is merged, installation is:

```powershell
winget install sinnerk.envdoctor
```

## Validate before submitting

```powershell
winget validate --manifest .\packaging\winget
winget show --manifest .\packaging\winget
```

Both must succeed with no warnings. The second command prints the resolved installer
type, URL and hash, which is the quickest way to spot a stale hash.

To try the install from these local files, the machine must first allow it (administrator
PowerShell):

```powershell
winget settings --enable LocalManifestFiles
winget install --manifest .\packaging\winget
```

## Submit

1. Fork [`microsoft/winget-pkgs`](https://github.com/microsoft/winget-pkgs).
2. Copy this folder to `manifests/s/sinnerk/envdoctor/<version>/` — note the layout:
   first letter of the publisher, lower case, then publisher, package, version.
3. Open a pull request. The repository's validation pipeline downloads the installer,
   checks the SHA256, and runs the package through a sandbox; it comments on the PR with
   the result.

`wingetcreate` automates the copy and the PR:

```powershell
wingetcreate submit .\packaging\winget
```

## Updating for a new release

Change four things in `sinnerk.envdoctor.installer.yaml`:

| Field | New value |
| --- | --- |
| `PackageVersion` (all three files) | the new version, without the `v` |
| `ReleaseDate` | release date, `YYYY-MM-DD` |
| `InstallerUrl` | `.../releases/download/v<version>/envdoctor.exe` |
| `InstallerSha256` | `Get-FileHash -Algorithm SHA256 envdoctor.exe` |

Then copy the folder to `manifests/s/sinnerk/envdoctor/<new-version>/` and open a new PR.
The old version's folder stays, so users on it keep working.

## Why the fields look the way they do

- `InstallerType: portable` with a direct `.exe` URL is the pattern used by other
  single-binary Go tools in winget-pkgs (for example `MikeFarah.yq`); no archive is needed.
- `Commands: [envdoctor]` is what puts `envdoctor` on `PATH` after installation.
- `Moniker: envdoctor` makes `winget install envdoctor` resolve too.
- `License: MIT` matches the repository's `LICENSE`.
