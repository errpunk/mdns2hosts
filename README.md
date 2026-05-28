# mdns2hosts

[![Go Version](https://img.shields.io/badge/Go-1.24%2B-blue)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)
[![Coverage](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/errpunk/mdns2hosts/main/coverage-badge.json)](https://github.com/errpunk/mdns2hosts/actions)

Sync mDNS `.local` names to the Windows `hosts` file — automatically.

mdns2hosts resolves mDNS names directly via multicast DNS (bypassing system DNS) and writes the resulting IPv4 addresses into a managed block of the Windows hosts file. Other entries in the hosts file are never touched.

## Installation

### Download prebuilt binary

Go to [Releases](https://github.com/errpunk/mdns2hosts/releases) and download `mdns2hosts.exe`.

### Build from source

```powershell
git clone https://github.com/errpunk/mdns2hosts.git
cd mdns2hosts
go build -o mdns2hosts.exe .
```

Requires Go 1.24+.

### Cross-compile from macOS/Linux

```bash
GOOS=windows GOARCH=amd64 go build -o mdns2hosts.exe .
```

## Usage

### Sync mDNS names to hosts

```powershell
# Single name
mdns2hosts sync supercow.local

# Multiple names
mdns2hosts sync supercow.local devbox.local printer.local
```

### Watch for IP changes continuously

```powershell
mdns2hosts watch supercow.local --interval 30s
```

Runs until interrupted (Ctrl+C). Updates hosts whenever an IP changes.

### Remove managed entries

```powershell
mdns2hosts clean
```

Removes all entries between the managed block markers.

### Run as a Windows service

```powershell
# Install (requires Administrator)
mdns2hosts install-service --name supercow.local --interval 30s

# Start the service
sc start mdns2hosts

# Stop and remove
mdns2hosts uninstall-service
```

## How it works

1. **mDNS query**: Sends a multicast DNS A-record query directly to `224.0.0.251:5353`. No system DNS resolution involved.
2. **Hosts update**: Writes resolved IPs into a managed block of the Windows hosts file (`C:\Windows\System32\drivers\etc\hosts`).
3. **Managed block**: Only touches lines between the markers — everything else is preserved exactly.

```text
# BEGIN mdns2hosts
192.168.1.88 supercow.local
# END mdns2hosts
```

Updates are atomic (write to temp file, rename over real file). Line endings are CRLF.

## Requirements

- Windows 10+ or Windows Server 2016+
- Administrator privileges (for `install-service`, `uninstall-service`, and writing to the hosts file)

## License

MIT
