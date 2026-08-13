# Installation Guide

## Release binary installation
The simplest way to install HowlFrame is to download a pre-compiled release binary.

1. Go to the [GitHub Releases](https://github.com/howlcipher/howlframe/releases) page.
2. Download the appropriate `.tar.gz` or `.zip` file for your platform.
3. Extract the archive.
4. Move the `howlframe` binary to a directory in your `$PATH`.

### Linux
```bash
wget https://github.com/howlcipher/howlframe/releases/download/v0.1.0/howlframe_0.1.0_linux_amd64.tar.gz
tar -xzf howlframe_0.1.0_linux_amd64.tar.gz
sudo mv howlframe /usr/local/bin/
```

### macOS
```bash
curl -LO https://github.com/howlcipher/howlframe/releases/download/v0.1.0/howlframe_0.1.0_darwin_amd64.tar.gz
tar -xzf howlframe_0.1.0_darwin_amd64.tar.gz
mv howlframe /usr/local/bin/
```

### Windows
Download `howlframe_0.1.0_windows_amd64.zip`, extract it, and place `howlframe.exe` in a directory included in your system `%PATH%`.

## Build from source
If you have Go 1.21 or newer installed, you can build from source:

```bash
git clone https://github.com/howlcipher/howlframe.git
cd howlframe
go build -o howlframe howlframe.go
```

Move the resulting `howlframe` binary to your PATH.

## Go install
*(Not officially supported or tested for global installation at this time due to layout requirements.)*
