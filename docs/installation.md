# Installation Guide

## Release binary installation
The simplest way to install HowlFrame is to download a pre-compiled release binary.

1. Go to the [GitHub Releases](https://github.com/howlcipher/howlframe/releases) page.
2. Download the archive for your platform and `SHA256SUMS`. Archive names keep the `v` of the tag:
   `howlframe_v0.2.0_<os>_<arch>.tar.gz`, or `.zip` on Windows.
3. Verify the checksum, extract the archive (it contains the single executable `howlframe`, or `howlframe.exe`), and
   move the executable to a directory in your `PATH`.
4. Run `howlframe version`. It prints the release version and the bytecode format it writes:
   `HowlFrame 0.2.0` and `HFBC format: 2`.

| Archive | Platform | Status for v0.2.0 |
|---|---|---|
| `howlframe_v0.2.0_linux_amd64.tar.gz` | Linux x86-64 | Built, executed and acceptance-tested |
| `howlframe_v0.2.0_linux_arm64.tar.gz` | Linux ARM64 | Built; executed only under user-mode emulation |
| `howlframe_v0.2.0_darwin_amd64.tar.gz` | macOS Intel | Built; **not executed** |
| `howlframe_v0.2.0_darwin_arm64.tar.gz` | macOS Apple silicon | Built; **not executed** |
| `howlframe_v0.2.0_windows_amd64.zip` | Windows x86-64 | Built; **not executed** |

The binaries are not code-signed. On macOS and Windows, governed `exec` contains fewer descendant processes than on
Linux (see `SECURITY.md`); run programs that need `process:exec` on Linux.

### Linux
```bash
VERSION=v0.2.0
curl -fLO https://github.com/howlcipher/howlframe/releases/download/${VERSION}/howlframe_${VERSION}_linux_amd64.tar.gz
curl -fLO https://github.com/howlcipher/howlframe/releases/download/${VERSION}/SHA256SUMS
sha256sum --check --ignore-missing SHA256SUMS
tar -xzf howlframe_${VERSION}_linux_amd64.tar.gz
sudo install -m 0755 howlframe /usr/local/bin/howlframe
howlframe version
```
Use `linux_arm64` in both names on ARM64.

### macOS
```bash
VERSION=v0.2.0
ARCH=arm64   # amd64 on Intel
curl -fLO https://github.com/howlcipher/howlframe/releases/download/${VERSION}/howlframe_${VERSION}_darwin_${ARCH}.tar.gz
curl -fLO https://github.com/howlcipher/howlframe/releases/download/${VERSION}/SHA256SUMS
shasum -a 256 --check --ignore-missing SHA256SUMS
tar -xzf howlframe_${VERSION}_darwin_${ARCH}.tar.gz
sudo install -m 0755 howlframe /usr/local/bin/howlframe
howlframe version
```
A binary downloaded through a browser may be quarantined by Gatekeeper because it is not signed.

### Windows
Download `howlframe_v0.2.0_windows_amd64.zip` and `SHA256SUMS`, check the archive with
`Get-FileHash howlframe_v0.2.0_windows_amd64.zip -Algorithm SHA256` against the matching line of `SHA256SUMS`, extract
it, and place `howlframe.exe` in a directory included in your `%PATH%`.

### Older releases
v0.1.0 archives use the same naming, for example `howlframe_v0.1.0_linux_amd64.tar.gz`. A v0.1.0 binary writes and
reads only HFBC format 1 and rejects artifacts built by v0.2.0; see
[artifact compatibility](reference/artifact_compatibility.md).

## Build from source
If you have Go 1.25 or newer installed, you can build from source. `go.mod` requires 1.25 for the race-free filesystem layer
(`os.Root.MkdirAll`, `Rename` and `Chmod`). With an older Go and the default `GOTOOLCHAIN=auto`, `go build` downloads 1.25;
with `GOTOOLCHAIN=local` it fails.

```bash
git clone https://github.com/howlcipher/howlframe.git
cd howlframe
go build -o howlframe howlframe.go
```

Move the resulting `howlframe` binary to your PATH.

## Go install
*(Not officially supported or tested for global installation at this time due to layout requirements.)*
