# Clipd

Clipd is a small Go utility that lets a Linux machine send clipboard text and run programs on a Windows machine over a local network.

## What it does

1. Sends clipboard text from Linux to Windows.
2. Runs a Windows program with arguments.
3. Pipes stdin to a Windows program.
4. Resolves Linux paths to Windows drive paths using a mapping table.

## Components

1. Client built from `cmd/clipd` runs on Linux.
2. Server built from `cmd/server` runs on Windows and listens for requests.

## Build

Use Mage to build the binaries into the `bin` directory.

```bash
mage Build
```

On Linux this builds `bin/clipd`.
On Windows this builds `bin/clipd.exe` and `bin/server.exe`.

## Configuration

Clipd reads a JSON config file at `~/.clipd` on both Linux and Windows. Values support environment variable expansion.

### Example

```json
{
  "serverIP": "192.168.1.10",
  "serverPort": 5454,
  "password": "secret",
  "driveMappings": {
    "C:": "/mnt/c",
    "D:": "/mnt/d"
  }
}
```

## Usage

Send clipboard text from Linux to Windows:

```bash
echo hello | clipd
```

Resolve a Linux path to a Windows path:

```bash
clipd path ~/projects/demo
```

Run a Windows program:

```bash
clipd run notepad.exe
```

Pipe stdin to a Windows program:

```bash
printf "text" | clipd pipe clip.exe
```

## Notes

Requests are plain JSON over the network, so use it on a trusted network.
